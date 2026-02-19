package daemoncmd

import (
	"github.com/quailyquaily/mistermorph/internal/daemonruntime"
)

type TaskStatus = daemonruntime.TaskStatus

const (
	TaskQueued   TaskStatus = daemonruntime.TaskQueued
	TaskRunning  TaskStatus = daemonruntime.TaskRunning
	TaskPending  TaskStatus = daemonruntime.TaskPending
	TaskDone     TaskStatus = daemonruntime.TaskDone
	TaskFailed   TaskStatus = daemonruntime.TaskFailed
	TaskCanceled TaskStatus = daemonruntime.TaskCanceled
)

type SubmitTaskRequest = daemonruntime.SubmitTaskRequest

type SubmitTaskResponse = daemonruntime.SubmitTaskResponse

type TaskInfo = daemonruntime.TaskInfo

func ParseTaskStatus(raw string) (TaskStatus, bool) {
	return daemonruntime.ParseTaskStatus(raw)
}
