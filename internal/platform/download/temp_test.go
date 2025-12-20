package download

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestFetchWithTempFile_UsesCustomTempDir(t *testing.T) {
	// Create a custom temp dir for the test
	customTempDir, err := os.MkdirTemp("", "test_custom_temp")
	if err != nil {
		t.Fatalf("failed to create custom temp dir: %v", err)
	}
	defer os.RemoveAll(customTempDir)

	// Mock runner that checks if outPath is within customTempDir
	runner := func(ctx context.Context, rawURL, outPath, userAgent string) error {
		if !strings.HasPrefix(outPath, customTempDir) {
			t.Errorf("expected outPath to be in %s, got %s", customTempDir, outPath)
		}
		return nil
	}

	plan := DownloadPlan{
		Strategy:  StrategyDirect,
		OutputExt: "txt",
		URL:       "http://example.com",
	}

	outPath, err := fetchWithTempFile(context.Background(), plan, customTempDir, "ls", "agent", runner)

	if err != nil {
		t.Fatalf("fetchWithTempFile failed: %v", err)
	}

	if !strings.HasPrefix(outPath, customTempDir) {
		t.Errorf("expected outPath start with %s, got %s", customTempDir, outPath)
	}
}
