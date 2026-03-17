package main

import (
	"reflect"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func TestParseSummary(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name    string
		args    args
		want    []TableResult
		want1   []TableResult
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := ParseSummary(tt.args.filePath)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSummary() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSummary() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("ParseSummary() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_parseTableRow(t *testing.T) {
	type args struct {
		line    string
		section string
	}
	tests := []struct {
		name string
		args args
		want *TableResult
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseTableRow(tt.args.line, tt.args.section); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTableRow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPrintResults(t *testing.T) {
	type args struct {
		equivalent   []TableResult
		inconsistent []TableResult
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			PrintResults(tt.args.equivalent, tt.args.inconsistent)
		})
	}
}

func TestParseSyncDiffOutput(t *testing.T) {
	type args struct {
		summaryFile string
	}
	tests := []struct {
		name    string
		args    args
		want    *SyncDiffOutput
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSyncDiffOutput(tt.args.summaryFile)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSyncDiffOutput() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSyncDiffOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}
