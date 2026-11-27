package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/youngwoocho02/unity-cli/internal/client"
)

func TestTestCmd_RejectsInvalidMode(t *testing.T) {
	_, err := testCmd([]string{"--mode", "Foo"}, unusedSend(t), nil)
	if err == nil {
		t.Fatal("expected invalid mode error")
	}
	if !strings.Contains(err.Error(), "EditMode or PlayMode") {
		t.Fatalf("error = %v, want mode validation", err)
	}
}

// setTestHome는 HOME/USERPROFILE을 임시 폴더로 돌려 실제 ~/.unity-cli를 건드리지 않게 한다.
// 커넥터가 쓰는 프로젝트별 status 폴더(~/.unity-cli/status/<projectKey>)를 만들어 돌려준다.
func setTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir := filepath.Join(home, ".unity-cli", "status", "0123456789abcdef")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create status dir: %v", err)
	}
	return dir
}

func runningResponse(t *testing.T, runID, resultsPath string) *client.CommandResponse {
	t.Helper()
	data, err := json.Marshal(map[string]string{"runId": runID, "resultsPath": resultsPath})
	if err != nil {
		t.Fatalf("failed to marshal running data: %v", err)
	}
	return &client.CommandResponse{Success: true, Message: "running", Data: data}
}

func TestPlayModeResultsPath_AcceptsProjectStatusFile(t *testing.T) {
	dir := setTestHome(t)
	want := filepath.Join(dir, "test-results-run-1.json")

	got, err := playModeResultsPath(runningResponse(t, "run-1", want), "run-1")
	if err != nil {
		t.Fatalf("playModeResultsPath returned error: %v", err)
	}
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestPlayModeResultsPath_Rejects(t *testing.T) {
	dir := setTestHome(t)
	home := filepath.Dir(filepath.Dir(filepath.Dir(dir)))

	cases := []struct {
		name  string
		runID string
		path  string
		want  string
	}{
		// 구 커넥터는 resultsPath를 주지 않는다. 옛 공용 폴더를 기다리지 않고 바로 실패한다.
		{"missing path", "run-1", "", "did not return a test results path"},
		{"other run id", "run-2", filepath.Join(dir, "test-results-run-2.json"), "run id"},
		{"other file name", "run-1", filepath.Join(dir, "test-results-run-9.json"), "unexpected test results path"},
		{"outside status dir", "run-1", filepath.Join(home, "test-results-run-1.json"), "unexpected test results path"},
		{"relative path", "run-1", "test-results-run-1.json", "unexpected test results path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := runningResponse(t, tc.runID, tc.path)
			_, err := playModeResultsPath(resp, "run-1")
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestPollTestResultsStopsWhenProjectInstanceDisappears(t *testing.T) {
	dir := setTestHome(t)

	// 유예 0이면 첫 resolve 실패를 바로 종료로 본다. 리로드 유예는 라이브/기본값에서만 쓴다.
	prevTimeout, prevGrace := testResultsPollTimeout, testResultsGoneGrace
	testResultsPollTimeout = time.Second
	testResultsGoneGrace = 0
	defer func() {
		testResultsPollTimeout = prevTimeout
		testResultsGoneGrace = prevGrace
	}()

	_, err := pollTestResults(filepath.Join(dir, "test-results-missing.json"), func() (*client.Instance, error) {
		return nil, fmt.Errorf("no Unity instance found for project: /projects/current")
	})
	if err == nil {
		t.Fatal("expected stopped editor error")
	}
	if !strings.Contains(err.Error(), "unity editor has stopped") {
		t.Fatalf("expected stopped editor error, got %v", err)
	}
}

func TestPollTestResultsIgnoresTransientMissingInstance(t *testing.T) {
	dir := setTestHome(t)

	prevTimeout, prevGrace := testResultsPollTimeout, testResultsGoneGrace
	testResultsPollTimeout = 3 * time.Second
	testResultsGoneGrace = time.Minute
	defer func() {
		testResultsPollTimeout = prevTimeout
		testResultsGoneGrace = prevGrace
	}()

	resultsPath := filepath.Join(dir, "test-results-transient-1.json")
	data, err := json.Marshal(client.CommandResponse{Success: true, Message: "done"})
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	// 첫 폴링은 인스턴스가 없다. 결과 파일이 생기면 리로드가 끝난 것으로 보고 성공해야 한다.
	n := 0
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = os.WriteFile(resultsPath, data, 0644)
	}()

	resp, err := pollTestResults(resultsPath, func() (*client.Instance, error) {
		n++
		if n < 3 {
			return nil, fmt.Errorf("no Unity instance found for project: /projects/current")
		}
		return &client.Instance{}, nil
	})
	if err != nil {
		t.Fatalf("pollTestResults returned error: %v", err)
	}
	if resp.Message != "done" {
		t.Fatalf("Message = %q, want done", resp.Message)
	}
}

func TestPollTestResultsReadsResultFile(t *testing.T) {
	dir := setTestHome(t)

	resultsPath := filepath.Join(dir, "test-results-run-1.json")
	data, err := json.Marshal(client.CommandResponse{Success: true, Message: "done"})
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}
	if err := os.WriteFile(resultsPath, data, 0644); err != nil {
		t.Fatalf("failed to write results: %v", err)
	}

	resp, err := pollTestResults(resultsPath, nil)
	if err != nil {
		t.Fatalf("pollTestResults returned error: %v", err)
	}
	if resp.Message != "done" {
		t.Fatalf("Message = %q, want done", resp.Message)
	}
	if _, err := os.Stat(resultsPath); !os.IsNotExist(err) {
		t.Fatalf("results file should be removed after reading, stat err = %v", err)
	}
}

// 옛 공용 폴더(status/ 바로 아래)에 같은 runId 파일이 있어도 읽지 않는다.
// 다른 에디터가 그 자리에 쓴 결과를 이 실행의 결과로 받지 않기 위해서다.
func TestPollTestResultsIgnoresSharedStatusDir(t *testing.T) {
	dir := setTestHome(t)

	prevTimeout, prevGrace := testResultsPollTimeout, testResultsGoneGrace
	testResultsPollTimeout = time.Second
	testResultsGoneGrace = 0
	defer func() {
		testResultsPollTimeout = prevTimeout
		testResultsGoneGrace = prevGrace
	}()

	data, err := json.Marshal(client.CommandResponse{Success: true, Message: "other project"})
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}
	shared := filepath.Join(filepath.Dir(dir), "test-results-run-1.json")
	if err := os.WriteFile(shared, data, 0644); err != nil {
		t.Fatalf("failed to write shared results: %v", err)
	}

	_, err = pollTestResults(filepath.Join(dir, "test-results-run-1.json"), nil)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout, got %v", err)
	}
}
