package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"os"
	"os/exec"
	"strings"
	"sync"
)

var version string

func init() {
	log.SetOutput(os.Stderr)
	log.SetLevel(log.WarnLevel)
}

func main() {
	app := cli.NewApp()
	app.Name = "backup-projects"
	app.Usage = "Backup projects"
	app.Version = version
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "backup-to",
			Usage: "Path to place backup archives.",
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Debug mode.",
		},
	}
	app.ArgsUsage = "[project dir]..."
	app.Before = func(c *cli.Context) (err error) {
		if c.GlobalBool("debug") {
			log.SetLevel(log.DebugLevel)
		}
		return
	}
	app.Action = func(c *cli.Context) {
		projectDirs := c.Args()
		log.WithField("givenProjectDirs", projectDirs).Debug("Project directories are given")
		if len(projectDirs) == 0 {
			log.Error("Projects dirs must be specified.")
			log.Exit(1)
		}
		backupDir := c.String("backup-to")
		if len(backupDir) == 0 {
			log.Error("Backup directory must be specified.")
			log.Exit(1)
		}
		if !directoryExists(backupDir) {
			log.Error("Backup directory does not exist.")
			log.Exit(1)
		}
		wg := &sync.WaitGroup{} // WaitGroupの値を作る
		for _, projectDir := range projectDirs {
			wg.Add(1)
			go func(projectDir string) {
				makeBackup(projectDir, backupDir)
				wg.Done()
			}(projectDir)
		}
		wg.Wait()
	}
	app.Run(os.Args)
}

func makeBackup(projectDir string, backupDir string) {
	if !directoryExists(projectDir) {
		log.Warningf("Project directory does not found: %s", projectDir)
		return
	}
	archiveName := strings.Replace(projectDir, "/", ":", -1) + ".zip"
	archiveFilename := backupDir + "/" + archiveName
	log.WithField("project-dir", projectDir).Info("Create backup")
	err := exec.Command("zip", "-r", archiveFilename, projectDir).Run()
	if err != nil {
		log.WithError(err).Warningf("Unable to compress: %s", projectDir)
		return
	}
	log.WithField("archive", archiveFilename).Info("Backup created")
}

func directoryExists(dir string) bool {
	if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
		return true
	}
	return false
}
