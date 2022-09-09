package cmd

import (
	"bitbucket.dentsplysirona.com/atopoc/auto-release-pr/utils"
	"encoding/base64"
	"fmt"
	"log"
	"os"
)

//https://bitbucket.dentsplysirona.com/scm/atopoc/cirrus-poc-gitops.git
const (
	bbBaseUrl   = "bitbucket.dentsplysirona.com/rest/api/1.0"
	repoBaseUrl = "bitbucket.dentsplysirona.com/scm"
	//username    = "USERNAME"
	//password    = "PASSWORD"
	username = "TEMPUSER"
	password = "BBTOKEN"
)

var bitBucketCredentialString string = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", os.Getenv(username), os.Getenv(password))))

var debugOn utils.Logger = utils.Logger{Debug: false}
var fatalError utils.Error = utils.Error{Fatal: true}

var logger = log.New(os.Stdout, "logger: ", log.Lshortfile)
