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
	"log"
	"os"

	"github.com/Fred78290/kubernetes-vmware-autoscaler/client"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/server"
	"github.com/Fred78290/kubernetes-vmware-autoscaler/types"
	glog "github.com/sirupsen/logrus"
)

var phVersion = "v0.0.0-unset"
var phBuildDate = ""

func main() {
	cfg := types.NewConfig()

	if err := cfg.ParseFlags(os.Args[1:], phVersion); err != nil {
		log.Fatalf("flag parsing error: %v", err)
	}

	ll, err := glog.ParseLevel(cfg.LogLevel)
	if err != nil {
		glog.Fatalf("failed to parse log level: %v", err)
	}

	glog.SetLevel(ll)

	if cfg.LogFormat == "json" {
		glog.SetFormatter(&glog.JSONFormatter{})
	}

	glog.Infof("config: %s", cfg)

	if cfg.DisplayVersion {
		glog.Infof("The current version is:%s, build at:%s", phVersion, phBuildDate)
	} else {
		generator := client.NewClientGenerator(cfg)

		if _, err := generator.NodeList(); err != nil {
			glog.Fatalf("Can't validate config, reason:%s", err)
		}

		if err := generator.CreateCRD(); err != nil {
			glog.Fatalf("Can't create CRD, reason:%s", err)
		}

		generator.WatchResources()

		server.StartServer(generator, cfg)
	}
}
