package controller

import (
	"qmhu/multi-cluster-cr/pkg/source"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

type Builder interface {
	InjectCluster(cluster source.Cluster)
	Construct() (controller.Controller, error)
}

type MultiClusterController struct {
	builders []Builder

	controllers []controller.Controller
}

func (c *MultiClusterController) AddBuilder(builder Builder) error {
	c.builders = append(c.builders, builder)
	return nil
}

func (c *MultiClusterController) CreateController(cluster source.Cluster) error {
	for _, builder := range c.builders {
		builder.InjectCluster(cluster)
		controller, err := builder.Construct()
		if err != nil {
			return err
		}

		c.controllers = append(c.controllers, controller)
	}
	return nil
}
