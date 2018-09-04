package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"os"
	"os/exec"
	"strings"
	"sync"
	"path"
	"io/ioutil"
	"crypto/md5"
	"io"
	"encoding/hex"
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
			log.Info("Backup directory does not exist.")
			if err := os.MkdirAll(backupDir, os.ModePerm); err != nil {
				log.Error("Unable to create backup directory.")
				log.Exit(1)
			}
		}
		tmpDir, err := ioutil.TempDir("", "backup-projects")
		if err != nil {
			log.Error("Unable to temporary backup directory.")
			log.Error(err)
			log.Exit(1)
		}
		log.Debugf("Temporary directory created: %s", tmpDir)
		defer os.RemoveAll(tmpDir)

		wg := &sync.WaitGroup{} // WaitGroupの値を作る
		for _, projectDir := range projectDirs {
			wg.Add(1)
			go func(projectDir string) {
				defer wg.Done()
				err2, newArchive := makeBackup(projectDir, tmpDir)
				if err2 != nil {
					return
				}
				newArchiveChecksum, err2 := md5sum(newArchive)
				if err2 != nil {
					log.WithField("err", err2).Error("Unable to get checksum: " + newArchive)
					return
				}
				log.
					WithField("input", newArchive).
					WithField("output", newArchiveChecksum).
					Debug("Calculate new archive checksum")

				// calculate previous archive checksum
				previousArchive := getBackupFilename(projectDir, backupDir)
				previousArchiveChecksum := "no-file"
				if _, err3 := os.Stat(previousArchive); err3 == nil {
					previousArchiveChecksum, err = md5sum(previousArchive)
					if err != nil {
						log.WithField("err", err).Error("Unable to get checksum: " + previousArchive)
						return
					}
				}
				log.
					WithField("input", previousArchive).
					WithField("output", previousArchiveChecksum).
					Debug("Calculate previous archive checksum")

				if newArchiveChecksum == previousArchiveChecksum {
					log.
						WithField("new-archive", newArchive).
						WithField("new-archive-checksum", newArchiveChecksum).
						WithField("previous-archive", previousArchive).
						WithField("previous-archive-checksum", previousArchiveChecksum).
						Info("New archive are not created")
				} else {
					temporaryArchive := newArchive
					backupFilename := previousArchive
					err = os.Rename(temporaryArchive, backupFilename)
					if err != nil {
						log.
							WithField("from", temporaryArchive).
							WithField("to", backupFilename).
							WithField("err", err).
							Error("Failed to move archive")
						return
					}
					log.
						WithField("filename", backupFilename).
						Info("Backup file created")
				}
			}(projectDir)
		}
		wg.Wait()
	}
	app.Run(os.Args)
}

func makeBackup(projectDir string, backupDir string) (err error, archiveFilename string) {
	if !directoryExists(projectDir) {
		log.Warningf("Project directory does not found: %s", projectDir)
		return
	}
	archiveFilename = getBackupFilename(projectDir, backupDir)
	parentDirOfProjectDir := path.Dir(projectDir)
	basenameOfProjectDir := path.Base(projectDir)
	log.
		WithField("project-dir", projectDir).
		WithField("parent-dir", parentDirOfProjectDir).
		WithField("basename", basenameOfProjectDir).
		Debug("Create archive")
	err = exec.Command("tar", "cf", archiveFilename, "-C", parentDirOfProjectDir, basenameOfProjectDir).Run()
	if err != nil {
		log.WithError(err).Warningf("Unable to compress: %s", projectDir)
		return
	}
	log.WithField("archive", archiveFilename).Debug("Archive created")
	return
}

func getBackupFilename(projectDir string, backupDir string) string {
	archiveName := strings.Replace(projectDir, "/", ":", -1) + ".tar"
	return backupDir + "/" + archiveName
}

func directoryExists(dir string) bool {
	if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
		return true
	}
	return false
}

func md5sum(filePath string) (result string, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return
	}

	result = hex.EncodeToString(hash.Sum(nil))
	return
}