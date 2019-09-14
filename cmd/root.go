/*
   Crossbar is a websocket relay
   Copyright (C) 2019 Timothy Drysdale <timothy.d.drysdale@gmail.com>

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU Affero General Public License as
   published by the Free Software Foundation, either version 3 of the
   License, or (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU Affero General Public License for more details.

   You should have received a copy of the GNU Affero General Public License
   along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/
package cmd

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"runtime/pprof"
	"sync"

	homedir "github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var bufferSize int64
var cfgFile string
var cpuprofile string
var host *url.URL
var listen string
var logFile string
var development bool

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

		fmt.Printf("Devmode? %v\n", development)
		if development {

			// development environment
			log.SetFormatter(&log.TextFormatter{})
			log.SetLevel(log.TraceLevel)
			log.SetOutput(os.Stdout)
			fmt.Println("Setting log output to stdout")

		} else {

			//production environment
			log.SetFormatter(&log.JSONFormatter{})
			log.SetLevel(log.WarnLevel)

			file, err := os.OpenFile("crossbar.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err == nil {
				log.SetOutput(file)
				fmt.Println("Setting log output to file")
			} else {
				log.Info("Failed to log to file, using default stderr")
			}

		}

		if cpuprofile != "" {
			f, err := os.Create(cpuprofile)
			if err != nil {
				log.Fatal(err)
			}
			pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
		}

		var wg sync.WaitGroup
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		messagesToDistribute := make(chan message, 10) //TODO make buffer length configurable
		var topics topicDirectory
		topics.directory = make(map[string][]clientDetails)
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
		}
		fmt.Printf("listen %v, host %v\n", listen, host)

		wg.Add(3)
		//func HandleConnections(closed <-chan struct{}, wg *sync.WaitGroup, clientActionsChan chan clientAction, messagesFromMe chan message)
		go HandleConnections(closed, &wg, clientActionsChan, messagesToDistribute, host)

		//func HandleMessages(closed <-chan struct{}, wg *sync.WaitGroup, topics *topicDirectory, messagesChan <-chan message)
		//go HandleMessages(closed, &wg, &topics, messagesToDistribute)

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
	rootCmd.PersistentFlags().StringVar(&cpuprofile, "cpuprofile", "", "write cpu profile to file")
	rootCmd.PersistentFlags().BoolVar(&development, "dev", false, "development environment")

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
