package bookkeeper

import (
	"github.com/akuity/bookkeeper/internal/git"
	log "github.com/sirupsen/logrus"
)

type renderRequestContext struct {
	logger       *log.Entry
	request      RenderRequest
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
