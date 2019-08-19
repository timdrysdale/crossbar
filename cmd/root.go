/*
Copyright © 2019 Timothy Drysdale <timothy.d.drysdale@gmail.com>

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
package cmd

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"sync"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var cfgFile string
var listen string
var bufferSize int64
var logFile string
var host url.URL

/* configuration

bufferSize
muxBufferLength (for main message queue into the mux)
clientBufferLength (for each client's outgoing channel)

*/

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "crossbar",
	Short: "websocket relay with topics",
	Long: `Crossbar is a websocket relay with topics set by the URL path, 
and can handle binary and text messages.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {

		var wg sync.WaitGroup
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		messagesToDistribute := make(chan message, 10) //TODO make buffer length configurable
		var topics topicDirectory
		clientActionsChan := make(chan clientAction)
		closed := make(chan struct{})

		go func() {
			for _ = range c {

				close(closed)
				wg.Wait()
				os.Exit(1)

			}
		}()

		host, err := url.Parse(listen)
		if err != nil {
			panic(err)
		} else if host.Scheme == "" || host.Host == "" {
			fmt.Println("error: listen must be an absolute URL")
			return
		} else if host.Scheme != "ws" {
			fmt.Println("error: listen must begin with ws")
			return
		} else if host.Scheme != "wss" {
			fmt.Println("error: listen does not yet support wss; please use a reverse proxy")
			return
		}

		wg.Add(3)
		//func HandleConnections(closed <-chan struct{}, wg *sync.WaitGroup, clientActionsChan chan clientAction, messagesFromMe chan message)
		go HandleConnections(closed, &wg, clientActionsChan, messagesToDistribute)

		//func HandleMessages(closed <-chan struct{}, wg *sync.WaitGroup, topics *topicDirectory, messagesChan <-chan message)
		go HandleMessages(closed, &wg, &topics, messagesToDistribute)

		//func HandleClients(closed <-chan struct{}, wg *sync.WaitGroup, topics *topicDirectory, clientActionsChan chan clientAction)
		go HandleClients(closed, &wg, &topics, clientActionsChan)
		wg.Wait()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.crossbar.yaml)")
	rootCmd.PersistentFlags().StringVar(&listen, "listen", "http://127.0.0.1:8080", "http://<ip>:<port> to listen on (default is http://127.0.0.1:8080)")
	rootCmd.PersistentFlags().Int64Var(&bufferSize, "buffer", 32768, "bufferSize in bytes (default is 32,768)")
	rootCmd.PersistentFlags().StringVar(&logFile, "log", "", "log file (default is STDOUT)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".crossbar" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".crossbar")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
