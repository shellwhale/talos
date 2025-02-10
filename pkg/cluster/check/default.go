// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"context"
	"slices"
	"time"

	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

// checks is a map containing all our cluster checks defined once.
var checks = map[string]ClusterCheck{
	// PreBootSequenceChecks:
	"etcd to be healthy": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("etcd to be healthy", func(ctx context.Context) error {
			return ServiceHealthAssertion(ctx, cluster, "etcd", WithNodeTypes(machine.TypeInit, machine.TypeControlPlane))
		}, 5*time.Minute, 5*time.Second)
	},
	"etcd members to be consistent across nodes": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("etcd members to be consistent across nodes", func(ctx context.Context) error {
			return EtcdConsistentAssertion(ctx, cluster)
		}, 5*time.Minute, 5*time.Second)
	},
	"etcd members to be control plane nodes": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("etcd members to be control plane nodes", func(ctx context.Context) error {
			return EtcdControlPlaneNodesAssertion(ctx, cluster)
		}, 5*time.Minute, 5*time.Second)
	},
	"apid to be ready": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("apid to be ready", func(ctx context.Context) error {
			return ApidReadyAssertion(ctx, cluster)
		}, 5*time.Minute, 5*time.Second)
	},
	"all nodes memory sizes": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("all nodes memory sizes", func(ctx context.Context) error {
			return AllNodesMemorySizes(ctx, cluster)
		}, 5*time.Minute, 5*time.Second)
	},
	"all nodes disk sizes": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("all nodes disk sizes", func(ctx context.Context) error {
			return AllNodesDiskSizes(ctx, cluster)
		}, 5*time.Minute, 5*time.Second)
	},
	"no diagnostics": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("no diagnostics", func(ctx context.Context) error {
			return NoDiagnostics(ctx, cluster)
		}, time.Minute, 5*time.Second)
	},
	"kubelet to be healthy": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("kubelet to be healthy", func(ctx context.Context) error {
			return ServiceHealthAssertion(ctx, cluster, "kubelet", WithNodeTypes(machine.TypeInit, machine.TypeControlPlane))
		}, 5*time.Minute, 5*time.Second)
	},
	"all nodes to finish boot sequence": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("all nodes to finish boot sequence", func(ctx context.Context) error {
			return AllNodesBootedAssertion(ctx, cluster)
		}, 5*time.Minute, 5*time.Second)
	},

	// K8sComponentsReadinessChecks:
	"all k8s nodes to report": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("all k8s nodes to report", func(ctx context.Context) error {
			return K8sAllNodesReportedAssertion(ctx, cluster)
		}, 5*time.Minute, 30*time.Second) // give more time per each attempt, as this check is going to build and cache kubeconfig
	},
	"all control plane static pods to be running": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("all control plane static pods to be running", func(ctx context.Context) error {
			return K8sControlPlaneStaticPods(ctx, cluster)
		}, 5*time.Minute, 5*time.Second)
	},
	"all control plane components to be ready": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("all control plane components to be ready", func(ctx context.Context) error {
			return K8sFullControlPlaneAssertion(ctx, cluster)
		}, 5*time.Minute, 5*time.Second)
	},

	// Additional checks used in DefaultClusterChecks:
	"all k8s nodes to report ready": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("all k8s nodes to report ready", func(ctx context.Context) error {
			return K8sAllNodesReadyAssertion(ctx, cluster)
		}, 10*time.Minute, 5*time.Second)
	},
	"kube-proxy to report ready": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("kube-proxy to report ready", func(ctx context.Context) error {
			present, replicas, err := DaemonSetPresent(ctx, cluster, "kube-system", "k8s-app=kube-proxy")
			if err != nil {
				return err
			}
			if !present {
				return conditions.ErrSkipAssertion
			}
			return K8sPodReadyAssertion(ctx, cluster, replicas, "kube-system", "k8s-app=kube-proxy")
		}, 5*time.Minute, 5*time.Second)
	},
	"coredns to report ready": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("coredns to report ready", func(ctx context.Context) error {
			present, replicas, err := DeploymentPresent(ctx, cluster, "kube-system", "k8s-app=kube-dns")
			if err != nil {
				return err
			}
			if !present {
				return conditions.ErrSkipAssertion
			}
			return K8sPodReadyAssertion(ctx, cluster, replicas, "kube-system", "k8s-app=kube-dns")
		}, 5*time.Minute, 5*time.Second)
	},
	"all k8s nodes to report schedulable": func(cluster ClusterInfo) conditions.Condition {
		return conditions.PollingCondition("all k8s nodes to report schedulable", func(ctx context.Context) error {
			return K8sAllNodesSchedulableAssertion(ctx, cluster)
		}, 5*time.Minute, 5*time.Second)
	},
}

// DefaultClusterChecks returns a set of default Talos cluster readiness checks.
func DefaultClusterChecks() []ClusterCheck {
	// Concatenate pre-boot, Kubernetes component, and additional checks.
	return slices.Concat(
		PreBootSequenceChecks(),
		K8sComponentsReadinessChecks(),
		[]ClusterCheck{
			// wait for all the nodes to report ready at k8s level
			checks["all k8s nodes to report ready"],
			// wait for kube-proxy to report ready
			checks["kube-proxy to report ready"],
			// wait for coredns to report ready
			checks["coredns to report ready"],
			// wait for all the nodes to be schedulable
			checks["all k8s nodes to report schedulable"],
		},
	)
}

// K8sComponentsReadinessChecks returns a set of K8s cluster readiness checks which are specific to the k8s components
// being up and running. This test can be skipped if the cluster is set to use a custom CNI, as the checks won't be healthy
// until the CNI is up and running.
func K8sComponentsReadinessChecks() []ClusterCheck {
	return []ClusterCheck{
		// wait for all the nodes to report in at k8s level
		checks["all k8s nodes to report"],
		// wait for k8s control plane static pods
		checks["all control plane static pods to be running"],
		// wait for HA k8s control plane
		checks["all control plane components to be ready"],
	}
}

// ExtraClusterChecks returns a set of additional Talos cluster readiness checks which work only for newer versions of Talos.
//
// ExtraClusterChecks can't be used reliably in upgrade tests, as older versions might not pass the checks.
func ExtraClusterChecks() []ClusterCheck {
	return []ClusterCheck{}
}

// PreBootSequenceChecks returns a set of Talos cluster readiness checks which are run before boot sequence.
var preBootSequenceChecks = []ClusterCheck{
	// wait for etcd to be healthy on all control plane nodes
	checks["etcd to be healthy"],
	// wait for etcd members to be consistent across nodes
	checks["etcd members to be consistent across nodes"],
	// wait for etcd members to be the control plane nodes
	checks["etcd members to be control plane nodes"],
	// wait for apid to be ready on all the nodes
	checks["apid to be ready"],
	// wait for all nodes to report their memory size
	checks["all nodes memory sizes"],
	// wait for all nodes to report their disk size
	checks["all nodes disk sizes"],
	// check diagnostics
	checks["no diagnostics"],
	// wait for kubelet to be healthy on all
	checks["kubelet to be healthy"],
	// wait for all nodes to finish booting
	checks["all nodes to finish boot sequence"],
}

func PreBootSequenceChecks() []ClusterCheck {
	return preBootSequenceChecks
}

func PreBootSequenceChecksFiltered(skips []string) []ClusterCheck {
	filtered := []ClusterCheck{}

	for name, check := range checks {
		// If the check name is in the `skips` list, exclude it
		if slices.Contains(skips, name) {
			continue
		}
		filtered = append(filtered, check)
	}

	return filtered
}
