package bookkeeper

import (
	"github.com/akuityio/bookkeeper/internal/git"
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
	branchConfig      branchConfig
	oldBranchMetadata branchMetadata
	newBranchMetadata branchMetadata
	prerenderedConfig []byte
	renderedConfig    []byte
	commit            commitContext
}

type commitContext struct {
	branch  string
	id      string
	message string
}
