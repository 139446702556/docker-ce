package service // import "github.com/docker/docker/integration/service"

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/swarm"
	"github.com/google/go-cmp/cmp"
	"github.com/gotestyourself/gotestyourself/assert"
	is "github.com/gotestyourself/gotestyourself/assert/cmp"
	"github.com/gotestyourself/gotestyourself/poll"
	"github.com/gotestyourself/gotestyourself/skip"
	"golang.org/x/net/context"
)

func TestInspect(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon())
	defer setupTest(t)()
	d := swarm.NewSwarm(t, testEnv)
	defer d.Stop(t)
	client := d.NewClientT(t)
	defer client.Close()

	var now = time.Now()
	var instances uint64 = 2
	serviceSpec := fullSwarmServiceSpec("test-service-inspect", instances)

	ctx := context.Background()
	resp, err := client.ServiceCreate(ctx, serviceSpec, types.ServiceCreateOptions{
		QueryRegistry: false,
	})
	assert.NilError(t, err)

	id := resp.ID
	poll.WaitOn(t, serviceContainerCount(client, id, instances))

	service, _, err := client.ServiceInspectWithRaw(ctx, id, types.ServiceInspectOptions{})
	assert.NilError(t, err)

	expected := swarmtypes.Service{
		ID:   id,
		Spec: serviceSpec,
		Meta: swarmtypes.Meta{
			Version:   swarmtypes.Version{Index: uint64(11)},
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	assert.Check(t, is.DeepEqual(service, expected, cmpServiceOpts()))
}

// TODO: use helpers from gotestyourself/assert/opt when available
func cmpServiceOpts() cmp.Option {
	const threshold = 20 * time.Second

	metaTimeFields := func(path cmp.Path) bool {
		switch path.String() {
		case "Meta.CreatedAt", "Meta.UpdatedAt":
			return true
		}
		return false
	}
	withinThreshold := cmp.Comparer(func(x, y time.Time) bool {
		delta := x.Sub(y)
		return delta < threshold && delta > -threshold
	})

	return cmp.FilterPath(metaTimeFields, withinThreshold)
}

func fullSwarmServiceSpec(name string, replicas uint64) swarmtypes.ServiceSpec {
	restartDelay := 100 * time.Millisecond
	maxAttempts := uint64(4)

	return swarmtypes.ServiceSpec{
		Annotations: swarmtypes.Annotations{
			Name: name,
			Labels: map[string]string{
				"service-label": "service-label-value",
			},
		},
		TaskTemplate: swarmtypes.TaskSpec{
			ContainerSpec: &swarmtypes.ContainerSpec{
				Image:           "busybox:latest",
				Labels:          map[string]string{"container-label": "container-value"},
				Command:         []string{"/bin/top"},
				Args:            []string{"-u", "root"},
				Hostname:        "hostname",
				Env:             []string{"envvar=envvalue"},
				Dir:             "/work",
				User:            "root",
				StopSignal:      "SIGINT",
				StopGracePeriod: &restartDelay,
				Hosts:           []string{"8.8.8.8  google"},
				DNSConfig: &swarmtypes.DNSConfig{
					Nameservers: []string{"8.8.8.8"},
					Search:      []string{"somedomain"},
				},
				Isolation: container.IsolationDefault,
			},
			RestartPolicy: &swarmtypes.RestartPolicy{
				Delay:       &restartDelay,
				Condition:   swarmtypes.RestartPolicyConditionOnFailure,
				MaxAttempts: &maxAttempts,
			},
			Runtime: swarmtypes.RuntimeContainer,
		},
		Mode: swarmtypes.ServiceMode{
			Replicated: &swarmtypes.ReplicatedService{
				Replicas: &replicas,
			},
		},
		UpdateConfig: &swarmtypes.UpdateConfig{
			Parallelism:     2,
			Delay:           200 * time.Second,
			FailureAction:   swarmtypes.UpdateFailureActionContinue,
			Monitor:         2 * time.Second,
			MaxFailureRatio: 0.2,
			Order:           swarmtypes.UpdateOrderStopFirst,
		},
		RollbackConfig: &swarmtypes.UpdateConfig{
			Parallelism:     3,
			Delay:           300 * time.Second,
			FailureAction:   swarmtypes.UpdateFailureActionPause,
			Monitor:         3 * time.Second,
			MaxFailureRatio: 0.3,
			Order:           swarmtypes.UpdateOrderStartFirst,
		},
	}
}

func serviceContainerCount(client client.ServiceAPIClient, id string, count uint64) func(log poll.LogT) poll.Result {
	return func(log poll.LogT) poll.Result {
		filter := filters.NewArgs()
		filter.Add("service", id)
		tasks, err := client.TaskList(context.Background(), types.TaskListOptions{
			Filters: filter,
		})
		switch {
		case err != nil:
			return poll.Error(err)
		case len(tasks) == int(count):
			return poll.Success()
		default:
			return poll.Continue("task count at %d waiting for %d", len(tasks), count)
		}
	}
}
