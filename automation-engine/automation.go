package main

type Automation struct {
	// automation consists of trigger, conditions, and actions
	Trigger   Trigger
	Condition Condition
	// Action    Action
}

type Condition struct {
}

// type Context struct {
// 	machineInfos map[string]server.Info
// }
