package lb

import (
	"context"
	"errors"
	"github.com/millisecond/adaptlb/config"
	"github.com/millisecond/adaptlb/model"
	"github.com/millisecond/adaptlb/util"
	"net"
	"strconv"
	"strings"
)

var activationMutex = &util.WrappedMutex{}

func Activate(activeConfig *config.Config, cfg *config.Config) error {
	ctx := context.Background()

	activationMutex.Lock(ctx)
	defer activationMutex.Unlock(ctx)

	initConfig(cfg)

	if activeConfig == nil {
		for _, frontend := range cfg.Frontends {
			err := addListener(frontend)
			if err != nil {
				return err
			}
		}
		activeConfig = cfg
		return nil
	}

	// we have an existing config, need to de-dupe
	removedFrontEnds := []*model.Frontend{}
	addedFrontends := []*model.Frontend{}

	// Update matches and collect FE's to remove
	for _, existing := range activeConfig.Frontends {
		found := false
		for _, toAdd := range cfg.Frontends {
			if toAdd.RowID == existing.RowID {
				toAdd.Listeners = existing.Listeners
				found = true
				break
			}
		}
		if !found {
			removedFrontEnds = append(removedFrontEnds, existing)
		}
	}

	// Find new FE's
	for _, toAdd := range cfg.Frontends {
		if toAdd.Type != "http" {
			if len(toAdd.ServerPools) > 1 {
				return errors.New("Cannot have multiple server pools for non-HTTP frontends.")
			} else if len(toAdd.ServerPools) == 0 {
				return errors.New("Must have a server pool to send traffic to.")
			}
		}

		found := false
		for _, existing := range activeConfig.Frontends {
			if toAdd.RowID == existing.RowID {
				// TODO changed ports
				toAdd.Listeners = existing.Listeners
				found = true
				break
			}
		}
		if !found {
			addedFrontends = append(addedFrontends, toAdd)
		}
	}

	// Stop old ones
	for _, fe := range removedFrontEnds {
		for _, listener := range *fe.Listeners {
			(*listener).Stop()
		}
	}

	// Start listening on new ones
	for _, fe := range addedFrontends {
		addListener(fe)
	}

	activeConfig.Frontends = cfg.Frontends

	return nil
}

// Initialize some temp structures through the config tree
func initConfig(cfg *config.Config) {
	for _, frontend := range cfg.Frontends {
		frontend.Listeners = &map[int]*model.Listener{}
		for _, pool := range frontend.ServerPools {
			pool.LiveServerMutex = &util.WrappedRWMutex{}
			pool.SharedLBState = &model.SharedLBState{
				Requests: 0,
			}
		}
	}
}

func addListener(frontend *model.Frontend) error {
	go healthcheck(frontend)

	switch frontend.Type {
	case "http":
	case "tcp":
		err := addTCPPort(frontend)
		if err != nil {
			return err
		}
	case "udp":
	default:
		return errors.New("Unknown config type: " + string(frontend.Type))
	}
	return nil
}

func portFromConn(c net.Conn) (int, error) {
	_, portS, err := net.SplitHostPort(c.LocalAddr().String())
	if err != nil {
		return -1, err
	}
	port, err := strconv.Atoi(portS)
	if err != nil {
		return -1, err
	}
	return port, nil
}

func parsePorts(portString string) ([]int, error) {
	ports := []int{}
	parts := strings.Split(portString, ",")
	for _, part := range parts {
		port, err := strconv.Atoi(part)
		if err != nil {
			return nil, err
		}
		ports = append(ports, port)
	}
	return ports, nil
}
