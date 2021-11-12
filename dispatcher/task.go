package dispatcher

import "errors"

type (
	// TaskStatus Job状态
	TaskStatus = int32

	// Task Task接口，表示对象可以外部启动、停止
	Task interface {
		Start() error
		Stop()
		Finish()
		Status() TaskStatus
	}

	// SubTask 子任务
	SubTask interface {
		Parent(ParentTask)
		Task
	}

	// ParentTask 父任务
	ParentTask interface {
		// Broadcast 建议Notify只是为sub task上报当前状态，不建议在parent task控制sub task状态（start）除外
		Notify(from SubTask, status TaskStatus)
		Task
	}
)

var (
	// ErrTaskInterrupted 任务被外部角色停止
	ErrTaskInterrupted = errors.New("current task was interrupted")
)

const (
	// StatusReady 任务未开始
	StatusReady TaskStatus = iota
	// StatusRunning 任务进行中
	StatusRunning
	// StatusPaused 任务暂停，可恢复
	StatusPaused
	// StatusFinished 任务正常完成
	StatusFinished
	// StatusStopped 任务已停止，非正常完成，不可恢复
	StatusStopped
)
