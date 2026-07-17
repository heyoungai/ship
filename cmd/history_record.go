package cmd

import (
	"errors"
	"fmt"

	"github.com/heyoungai/ship/internal"
)

// recordDeploymentResult 统一处理部署/回滚结果的历史写入。
func recordDeploymentResult(opErr error, version, action, result, note string, meta internal.HistoryMeta) error {
	recordErr := internal.RecordDeploymentWithMeta(version, action, result, note, meta)
	if recordErr == nil {
		return opErr
	}

	historyErr := fmt.Errorf("写入部署历史失败: %w", recordErr)
	if opErr != nil {
		return errors.Join(opErr, historyErr)
	}
	return historyErr
}
