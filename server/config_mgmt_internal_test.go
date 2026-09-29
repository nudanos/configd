// Copyright (c) 2019-2020, AT&T Intellectual Property. All rights reserved.
//
// SPDX-License-Identifier: LGPL-2.1-only

package server

var productionCallerCmdSetPrivs = callerCmdSetPrivs
var productionTmpDir = tmpDir
var productionConfigDir = configDir

func SetCallerCmdSetPrivs(set bool) {
	callerCmdSetPrivs = set
}

func GetProductionCallerCmdSetPrivs() bool {
	return productionCallerCmdSetPrivs
}

func SetTmpDir(dir string) {
	tmpDir = dir
}

func GetProductionTmpDir() string {
	return productionTmpDir
}

func SetConfigDir(dir string) {
	configDir = dir
}

func GetProductionConfigDir() string {
	return productionConfigDir
}

func SetCurrentConfigVersion(fn func() string) {
	currentConfigVersion = fn
}

func ResetCurrentConfigVersion() {
	currentConfigVersion = getCurrentConfigVersion
}

func SetConfigMgmtCmd(cmd string) {
	configMgmtCmd = cmd
}
