package main

import (
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

func TestRenderSyncDiffConfig(t *testing.T) {
	type args struct {
		config       *Config
		tableMapping *[]TableInfo
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
			if err := RenderSyncDiffConfig(tt.args.config, tt.args.tableMapping); (err != nil) != tt.wantErr {
				t.Errorf("RenderSyncDiffConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRenderDMSourceConfig(t *testing.T) {
	type args struct {
		config *Config
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
			if err := RenderDMSourceConfig(tt.args.config); (err != nil) != tt.wantErr {
				t.Errorf("RenderDMSourceConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRenderDMTaskConfig(t *testing.T) {
	type args struct {
		config       *Config
		tableMapping *[]TableInfo
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
			if err := RenderDMTaskConfig(tt.args.config, tt.args.tableMapping); (err != nil) != tt.wantErr {
				t.Errorf("RenderDMTaskConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
