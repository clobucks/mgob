package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mongodb/mongo-tools-common/db"

	"github.com/codeskyblue/go-sh"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/stefanprodan/mgob/pkg/config"
)

func Run(plan config.Plan, tmpPath string, storagePath, oplogTimestamp string, sess *db.SessionProvider) (Result, error) {
	t1 := time.Now()
	planDir := fmt.Sprintf("%v/%v", storagePath, plan.Name)

	var (
		archive, mlog string
		err           error
	)
	if plan.Target.Oplog {
		archive, mlog, err = dumpOplog(plan, tmpPath, t1.UTC(), oplogTimestamp)
	} else {
		archive, mlog, err = dump(plan, tmpPath, t1.UTC(), oplogTimestamp)
	}
	log.WithFields(log.Fields{
		"archive": archive,
		"mlog":    mlog,
		"planDir": planDir,
		"err":     err,
	}).Info("new dump")

	res := Result{
		Plan:      plan.Name,
		Timestamp: t1.UTC(),
		Status:    500,
	}
	_, res.Name = filepath.Split(archive)

	if err != nil {
		return res, err
	}

	if plan.Target.Oplog {
		ts, err := getCurrentOplogTime(sess)
		if err != nil {
			return res, errors.Wrapf(err, "getting current oplog timestamp from mongo")
		}
		res.OplogTimestamp.Time = ts.T
		res.OplogTimestamp.Order = ts.I
	}

	err = sh.Command("mkdir", "-p", planDir).Run()
	if err != nil {
		return res, errors.Wrapf(err, "creating dir %v in %v failed", plan.Name, storagePath)
	}

	fi, err := os.Stat(archive)
	if err != nil {
		return res, errors.Wrapf(err, "stat file %v failed", archive)
	}
	res.Size = fi.Size()

	err = sh.Command("mv", archive, planDir).Run()
	if err != nil {
		return res, errors.Wrapf(err, "moving file from %v to %v failed", archive, planDir)
	}

	// check if log file exists, is not always created
	if _, err := os.Stat(mlog); os.IsNotExist(err) {
		log.Debug("appears no log file was generated")
	} else {
		err = sh.Command("mv", mlog, planDir).Run()
		if err != nil {
			return res, errors.Wrapf(err, "moving file from %v to %v failed", mlog, planDir)
		}
	}

	if plan.Scheduler.Retention > 0 {
		err = applyRetention(planDir, plan.Scheduler.Retention)
		if err != nil {
			return res, errors.Wrap(err, "retention job failed")
		}
	}

	file := filepath.Join(planDir, res.Name)

	if plan.SFTP != nil {
		sftpOutput, err := sftpUpload(file, plan)
		if err != nil {
			return res, err
		} else {
			log.WithField("plan", plan.Name).Info(sftpOutput)
		}
	}

	if plan.S3 != nil {
		s3Output, err := s3Upload(file, plan)
		if err != nil {
			return res, err
		} else {
			log.WithField("plan", plan.Name).Infof("S3 upload finished %v", s3Output)
		}
	}

	if plan.GCloud != nil {
		gCloudOutput, err := gCloudUpload(file, plan)
		if err != nil {
			return res, err
		} else {
			log.WithField("plan", plan.Name).Infof("GCloud upload finished %v", gCloudOutput)
		}
	}

	if plan.Azure != nil {
		azureOutout, err := azureUpload(file, plan)
		if err != nil {
			return res, err
		} else {
			log.WithField("plan", plan.Name).Infof("Azure upload finished %v", azureOutout)
		}
	}

	t2 := time.Now()
	res.Status = 200
	res.Duration = t2.Sub(t1)
	return res, nil
}
