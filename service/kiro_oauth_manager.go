package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// KiroPollingTask 轮询任务
type KiroPollingTask struct {
	TaskID       string
	ClientID     string
	ClientSecret string
	DeviceCode   string
	Region       string
	ProxyURL     string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	Cancel       context.CancelFunc
	ResultChan   chan *KiroOAuthTokenResult
	ErrorChan    chan error
	// 缓存结果，允许多次读取
	Result *KiroOAuthTokenResult
	Error  error
	Mu     sync.RWMutex
}

// KiroOAuthManager 管理 Kiro OAuth 轮询任务
type KiroOAuthManager struct {
	tasks map[string]*KiroPollingTask
	mu    sync.RWMutex
}

var kiroManager = &KiroOAuthManager{
	tasks: make(map[string]*KiroPollingTask),
}

// GetKiroOAuthManager 获取全局管理器实例
func GetKiroOAuthManager() *KiroOAuthManager {
	return kiroManager
}

// StartPollingTask 启动后台轮询任务
func (m *KiroOAuthManager) StartPollingTask(task *KiroPollingTask) {
	m.mu.Lock()
	m.tasks[task.TaskID] = task
	m.mu.Unlock()

	common.SysLog(fmt.Sprintf("Kiro OAuth: 启动轮询任务 [%s]", task.TaskID))

	// 启动后台轮询
	go m.pollToken(task)
}

// GetTask 获取任务
func (m *KiroOAuthManager) GetTask(taskID string) (*KiroPollingTask, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, exists := m.tasks[taskID]
	return task, exists
}

// RemoveTask 移除任务
func (m *KiroOAuthManager) RemoveTask(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task, exists := m.tasks[taskID]; exists {
		if task.Cancel != nil {
			task.Cancel()
		}
		close(task.ResultChan)
		close(task.ErrorChan)
		delete(m.tasks, taskID)
	}
}

// pollToken 轮询获取 token
func (m *KiroOAuthManager) pollToken(task *KiroPollingTask) {
	// 延迟删除任务，给前端 3 分钟时间获取结果
	defer func() {
		time.Sleep(3 * time.Minute)
		m.RemoveTask(task.TaskID)
	}()

	ctx, cancel := context.WithDeadline(context.Background(), task.ExpiresAt)
	task.Cancel = cancel
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			err := fmt.Errorf("授权超时")
			task.Mu.Lock()
			task.Error = err
			task.Mu.Unlock()
			task.ErrorChan <- err
			common.SysLog(fmt.Sprintf("Kiro OAuth: 轮询超时 [%s]", task.TaskID))
			return

		case <-ticker.C:
			result, err := m.pollOnce(ctx, task)
			if err != nil {
				// 检查是否是 pending 状态
				if err.Error() == "authorization_pending" || err.Error() == "slow_down" {
					continue // 继续轮询
				}
				// 其他错误，停止轮询
				task.Mu.Lock()
				task.Error = err
				task.Mu.Unlock()
				task.ErrorChan <- err
				common.SysError(fmt.Sprintf("Kiro OAuth: 轮询失败 [%s]: %v", task.TaskID, err))
				return
			}

			// 成功获取 token，缓存结果
			task.Mu.Lock()
			task.Result = result
			task.Mu.Unlock()
			task.ResultChan <- result
			common.SysLog(fmt.Sprintf("Kiro OAuth: 轮询成功 [%s]", task.TaskID))
			return
		}
	}
}

// pollOnce 执行一次轮询
func (m *KiroOAuthManager) pollOnce(ctx context.Context, task *KiroPollingTask) (*KiroOAuthTokenResult, error) {
	flow := &KiroBuilderIDFlow{
		ClientID:     task.ClientID,
		ClientSecret: task.ClientSecret,
		DeviceCode:   task.DeviceCode,
	}

	return PollKiroBuilderIDTokenWithProxy(ctx, flow, task.Region, task.ProxyURL)
}
