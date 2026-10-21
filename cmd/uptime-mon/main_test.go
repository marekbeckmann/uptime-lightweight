package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLICommands(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "uptimemon-cli-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	binaryPath := filepath.Join(tempDir, "uptime-mon")

	// 1. Compile binary into tempDir
	cmdBuild := exec.Command("go", "build", "-o", binaryPath, "../../cmd/uptime-mon")
	if out, err := cmdBuild.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile uptime-mon: %v, output: %s", err, string(out))
	}

	configDir, _ := filepath.Abs("../../config")
	dataDir := filepath.Join(tempDir, "data")

	// 2. Test --config --validate
	cmdValidate := exec.Command(binaryPath, "--config", "--validate", "--config-dir", configDir)
	outValidate, err := cmdValidate.CombinedOutput()
	if err != nil {
		t.Fatalf("validation command failed: %v, output: %s", err, string(outValidate))
	}
	if !strings.Contains(string(outValidate), "Configuration valid. 0 errors found.") {
		t.Errorf("unexpected validation output: %s", string(outValidate))
	}

	// 3. Test --api --token --new
	cmdNew := exec.Command(binaryPath, "--api", "--token", "--new", "--name", "Automated Test", "--data-dir", dataDir)
	outNew, err := cmdNew.CombinedOutput()
	if err != nil {
		t.Fatalf("token new command failed: %v, output: %s", err, string(outNew))
	}
	outStr := string(outNew)
	if !strings.Contains(outStr, "Token ID:     tok_") || !strings.Contains(outStr, "Token Secret: sec_") {
		t.Errorf("unexpected token new output: %s", outStr)
	}

	// Extract token ID
	var tokenID string
	for _, line := range strings.Split(outStr, "\n") {
		if strings.HasPrefix(line, "Token ID:") {
			tokenID = strings.TrimSpace(strings.TrimPrefix(line, "Token ID:"))
			break
		}
	}
	if tokenID == "" {
		t.Fatalf("could not extract token ID from output: %s", outStr)
	}

	// 4. Test --api --token --list
	cmdList := exec.Command(binaryPath, "--api", "--token", "--list", "--data-dir", dataDir)
	outList, err := cmdList.CombinedOutput()
	if err != nil {
		t.Fatalf("token list command failed: %v, output: %s", err, string(outList))
	}
	if !strings.Contains(string(outList), tokenID) || !strings.Contains(string(outList), "ACTIVE") {
		t.Errorf("expected token %s with status ACTIVE in output: %s", tokenID, string(outList))
	}

	// 5. Test --api --token --revoke
	cmdRevoke := exec.Command(binaryPath, "--api", "--token", "--revoke", tokenID, "--data-dir", dataDir)
	outRevoke, err := cmdRevoke.CombinedOutput()
	if err != nil {
		t.Fatalf("token revoke command failed: %v, output: %s", err, string(outRevoke))
	}
	if !strings.Contains(string(outRevoke), "successfully revoked") {
		t.Errorf("unexpected revoke output: %s", string(outRevoke))
	}

	// 6. Test --api --token --list shows REVOKED
	cmdListRevoked := exec.Command(binaryPath, "--api", "--token", "--list", "--data-dir", dataDir)
	outListRevoked, err := cmdListRevoked.CombinedOutput()
	if err != nil {
		t.Fatalf("token list after revoke failed: %v", err)
	}
	if !strings.Contains(string(outListRevoked), "REVOKED") {
		t.Errorf("expected status REVOKED in output: %s", string(outListRevoked))
	}
}
