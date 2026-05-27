/*
 * API client — wraps Wails Go bindings.
 *
 * During `wails dev`, Wails auto-generates JS bindings at
 * frontend/wailsjs/go/desktop/App.js from the Go App struct.
 */

import {
	GetDevices,
	GetFreeDevices,
	GetTopology,
	GetRunningJobs,
	GetCompletedJobs,
	GetQueuedJobs,
	GetQueueLength,
	SubmitJob,
	KillJob,
	RemoveQueuedJob,
	UpdateQueuedPriority,
	GetDashboard
} from '../../wailsjs/go/desktop/App.js';

export {
	GetDevices,
	GetFreeDevices,
	GetTopology,
	GetRunningJobs,
	GetCompletedJobs,
	GetQueuedJobs,
	GetQueueLength,
	SubmitJob,
	KillJob,
	RemoveQueuedJob,
	UpdateQueuedPriority,
	GetDashboard
};
