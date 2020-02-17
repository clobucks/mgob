package backup

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/codeskyblue/go-sh"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/stefanprodan/mgob/pkg/config"
)

const timeFormat = "2006-01-02T15-04-05"

func dump(plan config.Plan, tmpPath string, ts time.Time, lastOplogTimestamp string) (string, string, error) {
	tsf := ts.Format(timeFormat)
	archive := fmt.Sprintf("%v/%v-%v.gz", tmpPath, plan.Name, tsf)

	mlog := fmt.Sprintf("%v/%v-%v.log", tmpPath, plan.Name, tsf)
	dump := fmt.Sprintf("mongodump --archive=%v --gzip ", archive)

	dump += fmt.Sprintf("--host %v --port %v ", plan.Target.Host, plan.Target.Port)

	if plan.Target.Username != "" {
		dump += fmt.Sprintf("-u %v ", plan.Target.Username)
	}

	if plan.Target.Password != "" {
		dump += fmt.Sprintf("-p %v ", plan.Target.Password)
	}

	if plan.Target.AuthSource != "" {
		dump += fmt.Sprintf("--authenticationDatabase %v ", plan.Target.AuthSource)
	}

	if plan.Target.Database != "" {
		dump += fmt.Sprintf("--db %v ", plan.Target.Database)
	}

	if plan.Target.Params != "" {
		dump += fmt.Sprintf("%v", plan.Target.Params)
	}

	// TODO: mask password
	log.Debugf("dump cmd: %v", dump)
	output, err := sh.Command("/bin/sh", "-c", dump).SetTimeout(time.Duration(plan.Scheduler.Timeout) * time.Minute).CombinedOutput()
	if err != nil {
		ex := ""
		if len(output) > 0 {
			ex = strings.Replace(string(output), "\n", " ", -1)
		}
		return "", "", errors.Wrapf(err, "mongodump log %v", ex)
	}
	logToFile(mlog, output)

	return archive, mlog, nil
}

func dumpOplog(plan config.Plan, tmpPath string, ts time.Time, lastOplogTimestamp string) (string, string, error) {
	if !plan.Target.Oplog {
		return "", "", fmt.Errorf("unexpected target setting: oplog value expected to be true")
	}
	tsf := ts.Format(timeFormat)
	archive := fmt.Sprintf("%v/%v-%v-initial-oplog.gz", tmpPath, plan.Name, tsf)

	initialBackup := lastOplogTimestamp == ""
	var dirToArchive string
	if !initialBackup {
		dirToArchive = fmt.Sprintf("%v/%v-%v-incremental-oplog", tmpPath, plan.Name, tsf)
		archive = fmt.Sprintf("%v/%v-%v-incremental-oplog.tar.gz", tmpPath, plan.Name, tsf)
	}

	mlog := fmt.Sprintf("%v/%v-%v.log", tmpPath, plan.Name, tsf)
	dump := "mongodump "
	if initialBackup {
		dump += fmt.Sprintf("--archive=%v --gzip ", archive)
	}

	dump += fmt.Sprintf("--host %v --port %v ", plan.Target.Host, plan.Target.Port)

	if plan.Target.Username != "" {
		dump += fmt.Sprintf("-u %v ", plan.Target.Username)
	}

	if plan.Target.Password != "" {
		dump += fmt.Sprintf("-p %v ", plan.Target.Password)
	}

	if plan.Target.AuthSource != "" {
		dump += fmt.Sprintf("--authenticationDatabase %v ", plan.Target.AuthSource)
	}

	if initialBackup {
		dump += "--oplog "
	} else {
		var ts timestamp
		if err := json.Unmarshal([]byte(lastOplogTimestamp), &ts); err != nil {
			return "", "", fmt.Errorf("unmarshalling oplogTimestamp=%s: %w", lastOplogTimestamp, err)
		}
		dump += `-d local -c oplog.rs --query '{"ts":{"$gt":{"$timestamp":{"t":` + strconv.Itoa(int(ts.Time)) +
			`,"i":` + strconv.Itoa(int(ts.Order)) + `}}}}' -o ` + dirToArchive + ` && tar -zcvf ` + archive + ` ` + dirToArchive
	}

	log.Debugf("dump cmd: %v", dump)
	output, err := sh.Command("/bin/sh", "-c", dump).SetTimeout(time.Duration(plan.Scheduler.Timeout) * time.Minute).CombinedOutput()
	if err != nil {
		ex := ""
		if len(output) > 0 {
			ex = strings.Replace(string(output), "\n", " ", -1)
		}
		return "", "", errors.Wrapf(err, "mongodump log %v", ex)
	}

	logToFile(mlog, output)

	return archive, mlog, nil
}

func logToFile(file string, data []byte) error {
	if len(data) > 0 {
		err := ioutil.WriteFile(file, data, 0644)
		if err != nil {
			return errors.Wrapf(err, "writing log %v failed", file)
		}
	}

	return nil
}

func applyRetention(path string, retention int) error {
	gz := fmt.Sprintf("cd %v && rm -f $(ls -1t *.gz | tail -n +%v)", path, retention+1)
	err := sh.Command("/bin/sh", "-c", gz).Run()
	if err != nil {
		return errors.Wrapf(err, "removing old gz files from %v failed", path)
	}

	log.Debug("apply retention")
	log := fmt.Sprintf("cd %v && rm -f $(ls -1t *.log | tail -n +%v)", path, retention+1)
	err = sh.Command("/bin/sh", "-c", log).Run()
	if err != nil {
		return errors.Wrapf(err, "removing old log files from %v failed", path)
	}

	return nil
}

// TmpCleanup remove files older than one day
func TmpCleanup(path string) error {
	rm := fmt.Sprintf("find %v -not -name \"mgob.db\" -mtime +%v -type f -delete", path, 1)
	err := sh.Command("/bin/sh", "-c", rm).Run()
	if err != nil {
		return errors.Wrapf(err, "%v cleanup failed", path)
	}

	return nil
}
