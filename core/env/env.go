/*
Copyright 2016 Medcl (m AT medcl.net)

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

package env

import (
	. "github.com/medcl/gopa/core/config"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
	log "github.com/cihub/seelog"
	"os"
)

type Env struct {
	//Logger logging.Logger
	*Registrar
	SystemConfig  *SystemConfig
	RuntimeConfig *RuntimeConfig
	Channels      *Channels
}

func Environment(sysConfig SystemConfig) *Env {
	//if logger == nil {
	//	logger = logging.NullLogger{}
	//}

	env := Env{}


	env.SystemConfig = &sysConfig
	config,err:= env.loadRuntimeConfig()
	if(err!=nil){
		panic(err)
	}
	env.RuntimeConfig = &config

	//override logging level
	if(len(sysConfig.LogLevel)>0){
		env.RuntimeConfig.LoggingConfig.Level = sysConfig.LogLevel
	}

	env.Channels = &Channels{}
	env.Channels.PendingFetchUrl = make(chan []byte, 10) //buffer number is 10
	env.Registrar = &Registrar{}
	//env.Logger = logger


	env.init()


	return &env
}

func (this *Env) loadRuntimeConfig()(RuntimeConfig,error) {

	var configFile="./gopa.yml"
	if(this.SystemConfig!=nil&&len(this.SystemConfig.ConfigFile)>0){
		configFile=this.SystemConfig.ConfigFile
	}

	//load external yaml config
	filename, _ := filepath.Abs(configFile)

	log.Debug("configFile:",filename)

	yamlFile, err := ioutil.ReadFile(filename)

	if err != nil {
		panic(err)
	}

	var config RuntimeConfig

	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		panic(err)
	}

	//override built-in config
	return config,err
}

func (this *Env) init()(error){
	os.MkdirAll(this.RuntimeConfig.PathConfig.Data, 0777)
	os.MkdirAll(this.RuntimeConfig.PathConfig.Log, 0777)
	return nil
}

func NullEnv() *Env {
	return 	&Env{}
}

type Channels struct {
	PendingFetchUrl chan []byte
}


//high priority config, init from the environment or startup, can't be changed
type SystemConfig struct {
	Version string `0.0.1`
	ConfigFile string `gopa.yml`
	LogLevel string `info`
}