package main


import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"fmt"
)


func main() {
	dsn := "root:1234Abcd@tcp(10.0.1.4:4000)/messagedb?parseTime=true"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Println("failed to open MySQL:", err)
		return
	}
	defer db.Close()

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		fmt.Println("failed to connect:", err)
		return
	}

	concurrency := 80
	totalLimit := 80
	chunk := (totalLimit + concurrency - 1) / concurrency

	resCh := make(chan string, totalLimit)
	errCh := make(chan error, concurrency)
	doneCh := make(chan struct{}, concurrency)

	goroutines := 0
	for i := 0; i < concurrency; i++ {
		offset := i * chunk
		limit := totalLimit - offset
		if limit <= 0 {
			break
		}
		if limit > chunk {
			limit = chunk
		}
		goroutines++
		go func(off, lim int) {

			// Run the first query; if the data exists, complete the process.
			// If the data does not exist, insert the data and check it again after completing the process.
			// The member id is taken from loop. It loops 10000 times and commits every 1000 rows.

			// Constants derived from the provided queries
			const (
				pushID           int64  = 19937
				createdAtCutoff  string = "2025-12-05 12:34:42.0"
				showStartAt      string = "2025-12-05 00:00:00.0"
				showStartAt02    string = "2025-12-08 00:00:00.0"
				showEndAt        string = "2025-12-07 23:59:59.0"
				updatedAt        string = "2025-12-05 12:49:33.678"
				messageType      int    = 4
				businessNo       string = "00000"
				pushMode         string = "022"
				messageContent   string = `{"showOnHomepage":0,"linkUrl":"type=screen0001","linkType":1,"informationCategory":1,"title":"1128消息公告","content":"test"}`
				loopCount        int    = 10000000
				commitBatchSize  int    = 10000
			)

			// Base values; include off to avoid ID/member collisions across goroutines
			baseMemberID := int64(2025032200000000 + off*loopCount)
			baseRowID := int64(3575551988 + off*loopCount)

			// Start transaction
			tx, err := db.Begin()
			if err != nil {
				errCh <- fmt.Errorf("begin tx error: %v", err)
				doneCh <- struct{}{}
				return
			}

//			selectSQL := `
//				SELECT id
//				FROM user_message
//				WHERE show_start_at IS NOT NULL
//				  AND show_end_at IS NOT NULL
//				  AND member_id = ?
//				  AND push_id = ?
//				  AND created_at >= ?
//				LIMIT 1
//			`

			insertSQL := `
				INSERT INTO user_message (
					id, member_id, message_type, business_no, push_all_id, message_content,
					readed, push_profile, deleted, push_mode, created_at, show_start_at,
					show_end_at, push_id, updated_at, unique_code
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`

// _, err = tx.Exec("SET RESOURCE GROUP rg2")
// if err != nil {
// 	_ = tx.Rollback()
// 	errCh <- fmt.Errorf("failed to set resource group: %v", err)
// 	doneCh <- struct{}{}
// 	return
// }

			inserted := 0
			for i := 0; i < loopCount; i++ {
				memberID := baseMemberID + int64(i)
				rowID := baseRowID + int64(i)

//				var existingID int64
//				err = tx.QueryRow(selectSQL, memberID, pushID, createdAtCutoff).Scan(&existingID)
//				if err != nil && err != sql.ErrNoRows {
//					_ = tx.Rollback()
//					errCh <- fmt.Errorf("select error (member_id=%d): %v", memberID, err)
//					doneCh <- struct{}{}
//					return
//				}
//
//				// If exists, skip insertion
//				if err == nil {
//					continue
//				}
                                var theShowStartAt string
                                if off < 6 {
				    theShowStartAt = showStartAt02
				} else {
				    theShowStartAt = showStartAt02
				}

				// Insert when not exists
				_, err = tx.Exec(
					insertSQL,
					rowID,               // id
					memberID,            // member_id
					messageType,         // message_type
					businessNo,          // business_no
					nil,                 // push_all_id
					messageContent,      // message_content
					0,                   // readed
					0,                   // push_profile
					0,                   // deleted
					pushMode,            // push_mode
					createdAtCutoff,     // created_at
  				        theShowStartAt,         // show_start_at
					showEndAt,           // show_end_at
					pushID,              // push_id
					updatedAt,           // updated_at
					nil,                 // unique_code
				)
				if err != nil {
					_ = tx.Rollback()
					errCh <- fmt.Errorf("insert error (member_id=%d): %v", memberID, err)
					doneCh <- struct{}{}
					return
				}

				inserted++

			//	// Verify after insert
			//	err = tx.QueryRow(selectSQL, memberID, pushID, createdAtCutoff).Scan(&existingID)
			//	if err != nil {
			//		_ = tx.Rollback()
			//		errCh <- fmt.Errorf("post-insert verify failed (member_id=%d): %v", memberID, err)
			//		doneCh <- struct{}{}
			//		return
			//	}

				// Commit every 1000 rows
				if inserted%commitBatchSize == 0 {
					if err = tx.Commit(); err != nil {
						errCh <- fmt.Errorf("commit error: %v", err)
						doneCh <- struct{}{}
						return
					}
					// Begin a new transaction for next batch
					tx, err = db.Begin()
					if err != nil {
						errCh <- fmt.Errorf("begin tx error (after commit): %v", err)
						doneCh <- struct{}{}
						return
					}
				}
			}

			// Final commit if there are pending changes
			if err = tx.Commit(); err != nil {
				errCh <- fmt.Errorf("final commit error: %v", err)
				doneCh <- struct{}{}
				return
			}

			// Send a concise summary message to avoid blocking on resCh
			resCh <- fmt.Sprintf("goroutine(off=%d, lim=%d): processed=%d inserted=%d", off, lim, 10000, inserted)
			doneCh <- struct{}{}
		}(offset, limit)
	}

	doneCount := 0
	for doneCount < goroutines {
		select {
		case msg := <-resCh:
			fmt.Println(msg)
		case err := <-errCh:
			fmt.Println(err)
			return
		case <-doneCh:
			doneCount++
		}
	}
}
