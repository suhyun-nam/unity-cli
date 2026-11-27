package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/youngwoocho02/unity-cli/internal/client"
)

type suppressWriter struct {
	w        io.Writer
	suppress string
}

// Write는 Unity 도메인 리로드 때 나오는 idle HTTP 경고를 숨긴다.
func (s *suppressWriter) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte(s.suppress)) {
		return len(p), nil
	}
	return s.w.Write(p)
}

// testCmd는 Unity Test Runner를 돌린다.
// EditMode는 응답에 결과가 바로 온다. PlayMode는 "running" 후 결과 파일을 폴링한다.
func testCmd(args []string, send sendFn, resolve instanceResolver) (*client.CommandResponse, error) {
	flags := parseSubFlags(args)

	mode := "EditMode"
	if m, ok := flags["mode"]; ok {
		mode = m
	}

	if mode != "EditMode" && mode != "PlayMode" {
		return nil, fmt.Errorf("--mode must be EditMode or PlayMode, got: %s", mode)
	}

	params := map[string]interface{}{
		"mode": mode,
	}
	runID := newTestRunID()
	params["runId"] = runID
	if filter, ok := flags["filter"]; ok {
		params["filter"] = filter
	}

	resp, err := send("run_tests", params)
	if err != nil {
		return nil, err
	}

	if !resp.Success && strings.Contains(resp.Message, "Unknown command") {
		return nil, fmt.Errorf(
			"'run_tests' is not available.\n" +
				"Install the Unity Test Framework package:\n" +
				"  Window > Package Manager > search 'Test Framework' > Install")
	}

	// EditMode: results returned directly in response
	if mode == "EditMode" {
		return resp, nil
	}

	// PlayMode: Unity returns "running", poll results file
	if resp.Message != "running" {
		return resp, nil
	}

	resultsPath, err := playModeResultsPath(resp, runID)
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(os.Stderr, "PlayMode tests running, waiting for results...")

	// Suppress "Unsolicited response received on idle HTTP channel" during domain reload
	original := log.Writer()
	log.SetOutput(&suppressWriter{w: os.Stderr, suppress: "Unsolicited response received on idle HTTP channel"})
	defer log.SetOutput(original)

	return pollTestResults(resultsPath, resolve)
}

// playModeResultsPath는 "running" 응답에서 커넥터가 결과를 쓸 파일 경로를 꺼낸다.
// 경로는 ~/.unity-cli/status/<projectKey>/test-results-<runId>.json이고, 프로젝트별 폴더라
// 다른 에디터의 결과가 이 파일에 써지지 않는다. CLI는 projectKey를 직접 계산하지 않는다
// (커넥터와 경로 표기가 달라 어긋날 수 있다).
// 읽은 뒤 지우는 파일이라, 이번 runId의 결과 파일이고 status 폴더 안일 때만 받는다.
func playModeResultsPath(resp *client.CommandResponse, runID string) (string, error) {
	var data struct {
		RunID       string `json:"runId"`
		ResultsPath string `json:"resultsPath"`
	}
	if len(resp.Data) > 0 {
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return "", fmt.Errorf("invalid PlayMode test response: %w", err)
		}
	}
	if data.ResultsPath == "" {
		return "", fmt.Errorf("connector did not return a test results path; update the Unity CLI Connector package")
	}
	if data.RunID != runID {
		return "", fmt.Errorf("connector returned run id %q, want %q", data.RunID, runID)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	statusRoot := filepath.Join(home, ".unity-cli", "status")
	path := filepath.Clean(data.ResultsPath)
	rel, err := filepath.Rel(statusRoot, path)
	if err != nil || !filepath.IsAbs(path) || rel == "." || strings.HasPrefix(rel, "..") ||
		filepath.Base(path) != fmt.Sprintf("test-results-%s.json", runID) {
		return "", fmt.Errorf("unexpected test results path from connector: %s", data.ResultsPath)
	}
	return path, nil
}

// newTestRunID는 이번 실행 전용 파일 이름에 쓰는 식별자다.
func newTestRunID() string {
	return fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
}

// PlayMode 결과 파일 대기. 테스트에서 timeout/grace를 줄여 바로 끝나게 한다.
var (
	testResultsPollTimeout = 10 * time.Minute
	// 도메인 리로드 동안 heartbeat가 잠깐 사라진다. 이 시간 안은 종료로 보지 않는다.
	testResultsGoneGrace = 30 * time.Second
)

// pollTestResults는 커넥터가 알려 준 resultsPath 파일이 생길 때까지 기다리고, 읽은 뒤 지운다.
// resolve 실패는 리로드일 수 있어 goneGrace 동안은 계속 폴링한다. 초를 짐작해 끝내지 않는다.
func pollTestResults(resultsPath string, resolve instanceResolver) (*client.CommandResponse, error) {
	deadline := time.Now().Add(testResultsPollTimeout)
	var goneSince time.Time

	for time.Now().Before(deadline) {
		time.Sleep(statusPollInterval)

		data, err := os.ReadFile(resultsPath)
		if err == nil {
			_ = os.Remove(resultsPath)
			var resp client.CommandResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				return nil, fmt.Errorf("failed to parse test results: %w", err)
			}
			return &resp, nil
		}

		if resolve != nil {
			if _, err := resolve(); err != nil {
				msg := err.Error()
				if strings.Contains(msg, "no Unity instances running") || strings.Contains(msg, "no Unity instance found for project") {
					if goneSince.IsZero() {
						goneSince = time.Now()
					}
					if testResultsGoneGrace == 0 || time.Since(goneSince) >= testResultsGoneGrace {
						return nil, fmt.Errorf("unity editor has stopped")
					}
					continue
				}
			} else {
				goneSince = time.Time{}
			}
		}
	}

	return nil, fmt.Errorf("timed out waiting for test results (%s)", testResultsPollTimeout)
}
