package validate

import (
	"testing"
)

func TestValidateCommand(t *testing.T) {
	// Skip integration tests in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
}

// テスト可能な単純な関数がvalidate.goにあれば、それらのテストも追加できます