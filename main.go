/*
Copyright 2018 Fred78290. https://github.com/Fred78290/

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"log"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/server"
)

var phVersion = "v0.0.0-unset"
var phBuildDate = ""

func main() {
	versionPtr := flag.Bool("version", false, "Give the version")
	savePtr := flag.String("save", "", "The file to persists the server")
	configPtr := flag.String("config", "/etc/default/AutoScaler-cluster-autoscaler.json", "The config for the server")
	flag.Parse()

	if *versionPtr {
		log.Printf("The current version is:%s, build at:%s", phVersion, phBuildDate)
	} else {
		server.StartServer(*savePtr, *configPtr)
	}
}
