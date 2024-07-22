// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT-0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"extension-lavasa/extension"
	"extension-lavasa/logsapi"
	agent "extension-lavasa/agent"

	"github.com/golang-collections/go-datastructures/queue"

	log "github.com/sirupsen/logrus"
)

// INITIAL_QUEUE_SIZE is the initial size set for the synchronous logQueue
const INITIAL_QUEUE_SIZE = 5

var (
	extensionName   = filepath.Base(os.Args[0]) // extension name has to match the filename
	printPrefix     = fmt.Sprintf("[%s]", extensionName)
	extensionClient = extension.NewClient(os.Getenv("AWS_LAMBDA_RUNTIME_API"))
)


func main() {
	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	logger := log.WithFields(log.Fields{"agent": extensionName})


	go func() {
		s := <-sigs
		cancel()
		println(printPrefix, "Received", s)
		println(printPrefix, "Exiting")
	}()

	res, err := extensionClient.Register(ctx, extensionName)
	if err != nil {
		panic(err)
	}

	logQueue := queue.New(INITIAL_QUEUE_SIZE)
	var logsStr string = ""
	flushLogQueue := func(force bool) {
	for !(logQueue.Empty() && (force || strings.Contains(logsStr, string(logsapi.RuntimeDone)))) {
		logs, err := logQueue.Get(1)
		if err != nil {
			logger.Error(printPrefix, err)
			return
		}
		logsStr = fmt.Sprintf("%v", logs[0])
		if err != nil {
			logger.Error(printPrefix, err)
			return
		}
	}
	}
	println(printPrefix, "Register response:", prettyPrint(res))
	// Create Logs API agent
	logsApiAgent, err := agent.NewHttpAgent(logQueue)
	if err != nil {
		logger.Fatal(err)
	}
	// Subscribe to logs API
	// Logs start being delivered only after the subscription happens.
	agentID := extensionClient.ExtensionID
	err = logsApiAgent.Init(agentID)
	if err != nil {
		logger.Fatal(err)
	}

	// Will block until shutdown event is received or cancelled via the context.
	for {
		select {
		case <-ctx.Done():
			return
		default:
			logger.Info(printPrefix, " Waiting for event...")
			// This is a blocking call
			res, err := extensionClient.NextEvent(ctx)
			if err != nil {
				logger.Info(printPrefix, "Error:", err)
				logger.Info(printPrefix, "Exiting")
				return
			}
			// Flush log queue in here after waking up
			flushLogQueue(false)
			// Exit if we receive a SHUTDOWN event
			if res.EventType == extension.Shutdown {
				logger.Info(printPrefix, "Received SHUTDOWN event")
				flushLogQueue(true)
				logger.Info(printPrefix, "Exiting")
				return
			}
		}
	}
}


func prettyPrint(v interface{}) string {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return ""
	}
	return string(data)
}
