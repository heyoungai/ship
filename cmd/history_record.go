package cmd

import (
	"errors"
	"fmt"
	"github.com/heyoungai/ship/internal"
)

// recordDeploymentResult 统一处理部署/回滚结果的历史写入，
// 避免主流程错误被历史写入问题吞掉，也避免历史写入失败被静默忽略。
func recordDeploymentResult(opErr error, version, action, result, note string) error {
	recordErr := internal.RecordDeployment(version, action, result, note)
	if recordErr == nil {
		return opErr
	}

	historyErr := fmt.Errorf("写入部署历史失败: %w", recordErr)
	if opErr != nil {
		return errors.Join(opErr, historyErr)
	}
	return historyErr
}
