package main

import (
	"testing"
)

func TestBurnBanStatus_String(t *testing.T) {
	tests := []struct {
		name string
		b    BurnBanStatus
		want string
	}{
		{
			name: "should return 'On' for ON status",
			b:    ON,
			want: "On",
		},
		{
			name: "should return 'Off' for OFF status",
			b:    OFF,
			want: "Off",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.b.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readPreviousStatus(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name    string
		args    args
		want    BurnBanStatus
		wantErr bool
	}{
		{
			name:    "should return ON for on file",
			args:    args{filePath: "test_data/bbstatus_ON.txt"},
			want:    ON,
			wantErr: false,
		},
		{
			name:    "should return OFF for on file",
			args:    args{filePath: "test_data/bbstatus_FF.txt"},
			want:    OFF,
			wantErr: false,
		},
		{
			name:    "should return err for non-existing file, and default to ON status",
			args:    args{filePath: "test_data/do_not_exist.txt"},
			want:    ON,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readPreviousStatus(tt.args.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("readPreviousStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("readPreviousStatus() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_writeStatus(t *testing.T) {
	type args struct {
		filePath string
		status   BurnBanStatus
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "should write Off status to file",
			args: args{
				filePath: "test_data/bbstatus_OFF.txt",
				status:   OFF,
			},
			wantErr: false,
		},
		{
			name: "should write On status to file",
			args: args{
				filePath: "test_data/bbstatus_ON.txt",
				status:   ON,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeStatus(tt.args.filePath, tt.args.status); (err != nil) != tt.wantErr {
				t.Errorf("writeStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
