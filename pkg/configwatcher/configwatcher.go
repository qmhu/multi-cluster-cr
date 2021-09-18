package configwatcher

import (
	"qmhu/multi-cluster-cr/pkg/config"
)

type EventType string

const (
	Added    EventType = "ADDED"
	Deleted  EventType = "DELETED"
)

// Event is a single event to a watched kubeconfig.
type Event struct {
	// Type defines the possible types of events.
	Type EventType

	// When Type is Added or Modified, the Config is the new state of the kubeconfig
	// When Type is Deleted, the Config is the latest version kubeconfig before deletion
	Config *config.NamedConfig
}

type ConfigWatcher interface {
	// Events returns a chan which can receive all the watched events.
	Events() <-chan Event

	// Errors returns a chan which can receive all the errors occur during watching.
	Errors() <-chan error

	// Stop will stop the inside watcher and release all the resources including event channel and error channel.
	Stop()
}
