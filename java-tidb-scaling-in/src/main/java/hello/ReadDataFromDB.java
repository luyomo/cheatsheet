package hello;

import com.zaxxer.hikari.HikariConfig;
import com.zaxxer.hikari.HikariDataSource;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;


import java.io.FileInputStream;
import java.io.IOException;
import java.util.Properties;

// Adapted from http://www.vogella.com/tutorials/MySQLJava/article.html

import java.sql.*;
import java.util.Date;
import java.sql.SQLException;

public class ReadDataFromDB {

    private Connection connect = null;
    private Statement statement = null;
    private ResultSet resultSet = null;

    private String url;
    private String user;
    private String passwd;
    private int parallel;
    private int loops;
    private int patternType;  // 1: insert statement
                             // 2: batch insert
			     // 3. multiple query insert

    public void readDataFromDB(String query) throws Exception {
        readConfig();

        HikariConfig config = new HikariConfig();
        // config.setJdbcUrl("jdbc:mysql://tidb.rmcmus1jpoor.clusters.tidb-cloud.com:4000/test?allowMultiQueries=true&autoReconnect=true");
        // config.setJdbcUrl("jdbc:mysql://10.0.2.7:4000/test?allowMultiQueries=true&autoReconnect=true");
        config.setJdbcUrl(url);
        config.setUsername(user);
        config.setPassword(passwd);

        // Optional settings
        config.setMaximumPoolSize(10);      // Maximum number of connections in the pool
        config.setMinimumIdle(2);           // Minimum number of idle connections
        config.setIdleTimeout(30000);       // Idle timeout in milliseconds
        config.setMaxLifetime(1800000);     // Maximum lifetime of a connection in milliseconds
        config.setConnectionTimeout(30000); // Timeout for getting a connection

        try (HikariDataSource dataSource = new HikariDataSource(config)) {
            executeQueriesInParallel(dataSource, query);
        } catch (Exception e) {
            System.out.println("Failed to connect the database ");
            e.printStackTrace();
        }
    }

    private void executeQueriesInParallel(HikariDataSource dataSource, String query) {
        // Create a thread pool
        ExecutorService executorService = Executors.newFixedThreadPool(parallel);

        resetDB(dataSource);

        // Submit tasks for parallel execution
        for (int idx =0 ; idx < parallel; idx++) {
            executorService.submit(() -> {
                try {
                    Connection connection = dataSource.getConnection();
                    Statement statement = connection.createStatement();

                    long threadId = Thread.currentThread().getId();
                    int numRow = 0;
                    while (numRow < loops){
                        try {
                            if (statement.isClosed()) {
                                connection = dataSource.getConnection();
                                statement = connection.createStatement();
		            }

			    switch (patternType){
                                case 1:                                        // Case 01: Simply select data from table
                                    if(numRow == 1) {
                                        System.out.println("Pattern 01: Use executeUpdate to insert data.");
				    }
			            statement.executeUpdate(String.format("INSERT INTO test01(col02, thread) VALUES ('%d', '%d')", numRow, threadId));
				    break;

                                    //// System.out.println("Executing query: " + query);
                                    //while (resultSet.next()) {
                                    //    System.out.println("thread: " + threadId + ", Column Value: " + resultSet.getInt("col01") + ", idx: " + numRow);
                                    //}

				case 2:                                        // Case 02: batch insert
                                    if(numRow == 1) {
                                        System.out.println("Pattern 02: Use batch to insert data.");
				    }
                                    statement.addBatch(String.format("INSERT INTO test01(col02, thread) VALUES ('%d', '%d')", numRow, threadId));
                                    statement.addBatch(String.format("INSERT INTO test01(col02, thread) VALUES ('%d', '%d')", numRow, threadId));
                                    statement.addBatch(String.format("INSERT INTO test01(col02, thread) VALUES ('%d', '%d')", numRow, threadId));
                                    statement.addBatch(String.format("INSERT INTO test01(col02, thread) VALUES ('%d', '%d')", numRow, threadId));

                                    int[] results = statement.executeBatch();
                                    // System.out.println("Batch executed. Rows affected: " + results.length);
				    break;

				case 3:                                         // Case 03: Execute the multiple queries insert
                                    if(numRow == 1) {
                                        System.out.println("Pattern 03: Use multiple query to insert data.");
				    }
                                    String multiQuery = 
                                          String.format("INSERT INTO test01(col02, thread) VALUES ('%d', '%d');", numRow, threadId)
                                        + String.format("INSERT INTO test01(col02, thread) VALUES ('%d', '%d');", numRow, threadId)
                                        + String.format("INSERT INTO test01(col02, thread) VALUES ('%d', '%d');", numRow, threadId)
                                        + String.format("SELECT count(*) as cnt FROM test01 where col02 = '%d' and thread = '%d';", numRow, threadId);

                                    boolean hasMoreResults = statement.execute(multiQuery); // Execute the query
                                    do {
                                        if (hasMoreResults) {
                                            try (ResultSet resultSet = statement.getResultSet()) {
                                                while (resultSet.next()) {
                                                    System.out.println("Result: " + resultSet.getString("cnt"));
                                                }
                                            }
                                        }
                                        hasMoreResults = statement.getMoreResults(); // Move to next result set
                                    } while (hasMoreResults);
				    break;
			    }
		        } catch (SQLException e) {
                            if (e.getSQLState().equals("08S01")) {
                                System.err.println(String.format("communication link failure: thread(%d)",  threadId));
                            }else {
				// Communications link failure
                                System.err.println("Error executing query: " + query);
                                e.printStackTrace();
                            }
		        }

                        numRow++;
                        Thread.sleep(1000);
			if (numRow % 50 == 0) {
                            System.out.println(String.format("Thread(%d), num(%d) is working", threadId, numRow));
                        }
                    }
                } catch (InterruptedException  e) {
                    Thread.currentThread().interrupt(); // Restore the interrupt status
                    System.out.println("Task interrupted.");
		} catch (Exception e) {
                    System.err.println("Error executing query: " + query);
                    e.printStackTrace();
                }
            });
        }

        // Shutdown the executor service
        executorService.shutdown();
        try {
            if (!executorService.awaitTermination(3600, TimeUnit.SECONDS)) {
                executorService.shutdownNow();
            }
        } catch (InterruptedException e) {
            executorService.shutdownNow();
            Thread.currentThread().interrupt();
        }
    }

    private void resetDB(HikariDataSource dataSource) {
        try { 
            Connection connection = dataSource.getConnection();
            Statement statement = connection.createStatement();

            statement.executeUpdate("drop table if exists test.test01");

            statement.executeUpdate("CREATE TABLE `test01` ( `col01` int(11) NOT NULL AUTO_INCREMENT, `col02` int(11) DEFAULT NULL, `thread` int(11) DEFAULT NULL, `created_at` timestamp DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (`col01`) ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin ");

   	} catch (Exception e) {
            e.printStackTrace();
        }
    }

    private void readConfig() {
        Properties properties = new Properties();

        try (FileInputStream input = new FileInputStream("config.properties")) {
            // Load properties file
            properties.load(input);

            // Access properties
            url         = properties.getProperty("database.url"     );
            user        = properties.getProperty("database.username");
            passwd      = properties.getProperty("database.password");
            parallel    = Integer.parseInt(properties.getProperty("run.parallel"     ));
            loops       = Integer.parseInt(properties.getProperty("run.loops"        ));
            patternType = Integer.parseInt(properties.getProperty("run.pattern_type" ));

            System.out.println("Database URL: " + url        );
            System.out.println("Username: "     + user       );
            System.out.println("Password: "     + passwd     );
            System.out.println("Parallel: "     + parallel   );
            System.out.println("Loops: "        + loops      );
            System.out.println("Pattern Type: " + patternType);
        } catch (IOException e) {
            e.printStackTrace();
        }
    }
}
