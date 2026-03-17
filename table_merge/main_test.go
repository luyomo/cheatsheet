package main

import (
	"reflect"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func Test_main(t *testing.T) {
	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			main()
		})
	}
}

func Test_generateGeneralRegex(t *testing.T) {
	type args struct {
		dataList               []string
		dataListShouldNotMatch []string
	}
	tests := []struct {
		name    string
		args    args
		want    *string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateGeneralRegex(tt.args.dataList, tt.args.dataListShouldNotMatch)
			if (err != nil) != tt.wantErr {
				t.Fatalf("generateGeneralRegex() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Errorf("generateGeneralRegex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_generateRegex(t *testing.T) {
	type args struct {
		tables               []string
		tablesShouldNotMatch []string
		mapPatterns          map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    *string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateRegex(tt.args.tables, tt.args.tablesShouldNotMatch, tt.args.mapPatterns)
			if (err != nil) != tt.wantErr {
				t.Fatalf("generateRegex() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Errorf("generateRegex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_rule_is_valid(t *testing.T) {
	type args struct {
		pattern              string
		tables               []string
		tablesShouldNotMatch []string
	}
	tests := []struct {
		name string
		args args
		want ToolReturn
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rule_is_valid(tt.args.pattern, tt.args.tables, tt.args.tablesShouldNotMatch); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rule_is_valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_fetch_table_def(t *testing.T) {
	type args struct {
		tableType      string
		tableStructure *[]TableInfo
		dbInfo         DBConnInfo
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := fetch_table_def(tt.args.tableType, tt.args.tableStructure, tt.args.dbInfo); (err != nil) != tt.wantErr {
				t.Errorf("fetch_table_def() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_readConfig(t *testing.T) {
	type args struct {
		fileName string
	}
	tests := []struct {
		name    string
		args    args
		want    Config
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readConfig(tt.args.fileName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("readConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_calculateSampleSize(t *testing.T) {
	type args struct {
		totalItems int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calculateSampleSize(tt.args.totalItems); got != tt.want {
				t.Errorf("calculateSampleSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sampleData(t *testing.T) {
	type args struct {
		data       []string
		sampleSize int
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sampleData(tt.args.data, tt.args.sampleSize); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sampleData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_splitTables(t *testing.T) {
	type args struct {
		tables []string
	}
	tests := []struct {
		name  string
		args  args
		want  []string
		want1 []string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := splitTables(tt.args.tables)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitTables() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("splitTables() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_fetchDumpingSourceData(t *testing.T) {
	type args struct {
		srcInstance  string
		srcSchema    string
		srcTable     string
		hasSourceCol bool
		hasSchemaCol bool
		hasTableCol  bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fetchDumpingSourceData(tt.args.srcInstance, tt.args.srcSchema, tt.args.srcTable, tt.args.hasSourceCol, tt.args.hasSchemaCol, tt.args.hasTableCol); got != tt.want {
				t.Errorf("fetchDumpingSourceData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetMaxID4IncreDiff(t *testing.T) {
	type args struct {
		config         Config
		tableStructure []TableInfo
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SetMaxID4IncreDiff(tt.args.config, tt.args.tableStructure); (err != nil) != tt.wantErr {
				t.Errorf("SetMaxID4IncreDiff() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
