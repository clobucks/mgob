package backup

import (
	"fmt"
	"strconv"

	mdb "github.com/mongodb/mongo-tools-common/db"
	"github.com/mongodb/mongo-tools-common/options"
	"github.com/mongodb/mongo-tools/mongodump"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// timestamp is a representation of a bson timestamp.
type timestamp struct {
	Time  uint32 `json:"time"`
	Order uint32 `json:"order"`
}

// InitSessionProvider creates session provider that establishes connection to mongodb.
func InitSessionProvider(user, password, host string, port int) (*mdb.SessionProvider, error) {
	opts := options.New(
		"mongodump",
		"built-without-version-string",
		"built-without-git-commit",
		"built-without-usage-string",
		options.EnabledOptions{Auth: true, Connection: true, Namespace: true, URI: true},
	)
	opts.Quiet = true
	inputOpts := mongodump.InputOptions{}
	opts.AddOptions(&inputOpts)
	outputOpts := mongodump.OutputOptions{}
	opts.AddOptions(&outputOpts)
	opts.URI.AddKnownURIParameters(options.KnownURIOptionsReadPreference)

	var args []string

	if host != "" {
		args = append(args, "--host", host)
	}
	if port != 0 {
		args = append(args, "--port", strconv.Itoa(port))
	}
	if user != "" {
		args = append(args, "-u", user)
	}
	if password != "" {
		args = append(args, "-p", password)
	}
	if _, err := opts.ParseArgs(args); err != nil {
		return nil, fmt.Errorf("parsing args: %w", err)
	}
	return mdb.NewSessionProvider(*opts)
}

// getCurrentOplogTime returns the most recent oplog entry's timestamp
func getCurrentOplogTime(sess *mdb.SessionProvider) (primitive.Timestamp, error) {
	mostRecentOplogEntry := mdb.Oplog{}
	var tempBSON bson.Raw

	err := sess.FindOne("local", "oplog.rs", 0, nil, &bson.M{"$natural": -1}, &tempBSON, 0)
	if err != nil {
		return primitive.Timestamp{}, fmt.Errorf("error getting recent oplog entry: %w", err)
	}
	err = bson.Unmarshal(tempBSON, &mostRecentOplogEntry)
	if err != nil {
		return primitive.Timestamp{}, err
	}
	return mostRecentOplogEntry.Timestamp, nil
}
