package render

import (
	log "github.com/sirupsen/logrus"

	"github.com/akuity/kargo-render/pkg/git"
)

type requestContext struct {
	logger       *log.Entry
	request      *Request
	repo         git.Repo
	source       sourceContext
	intermediate intermediateContext
	target       targetContext
}

type sourceContext struct {
	commit string
}

type intermediateContext struct {
	branchMetadata *branchMetadata
}

type targetContext struct {
	branchConfig         branchConfig
	oldBranchMetadata    branchMetadata
	newBranchMetadata    branchMetadata
	prerenderedManifests map[string][]byte
	renderedManifests    map[string][]byte
	commit               commitContext
}

type commitContext struct {
	branch            string
	oldBranchMetadata *branchMetadata
	id                string
	message           string
}
