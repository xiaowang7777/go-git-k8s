package k8s

import (
	"github.com/wang1137095129/go-git-k8s/config"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api"
	api_v1 "k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/labels"
	"k8s.io/client-go/1.5/pkg/selection"
	"k8s.io/client-go/1.5/pkg/util/sets"
	"k8s.io/client-go/1.5/rest"
	"time"
)

var (
	LabelKey   = "git_kubernetes_watch"
	LabelValue = "test"
	LabelWatch = "watch_repository"
)

type Handle struct {
	lastUpdateTime *time.Time
	client         *kubernetes.Clientset
}

func NewKubernetesHandle() (*Handle, error) {
	h := &Handle{}
	h.lastUpdateTime = new(time.Time)
	h.client = new(kubernetes.Clientset)
	if err := h.Load(); err != nil {
		return nil, err
	}
	return h, nil
}

func createSelector(c config.Config) (labels.Selector, error) {
	selector := labels.NewSelector()

	for key, value := range c.K8s.Labels {
		var newString = sets.NewString()
		newString.Insert(value)
		newRequirement, err := labels.NewRequirement(key, selection.In, newString)
		if err != nil {
			return nil, err
		}
		selector.Add(*newRequirement)
	}
	newString := sets.NewString(LabelValue)
	newRequirement, err := labels.NewRequirement(LabelKey, selection.In, newString)
	if err != nil {
		return nil, err
	}
	selector.Add(*newRequirement)

	repository := sets.NewString(c.Git.Repository)
	requirement, err := labels.NewRequirement(LabelWatch, selection.In, repository)
	if err != nil {
		return nil, err
	}
	selector.Add(*requirement)

	return selector, nil
}

func (h *Handle) Load() error {
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(inClusterConfig)
	if err != nil {
		return err
	}
	h.client = clientset
	return nil
}

func (h Handle) getPodList(c *config.Config) (*api_v1.PodList, error) {
	selector, err := createSelector(*c)
	if err != nil {
		return nil, err
	}
	if podList, err := h.client.Pods(c.K8s.Namespace).List(*(&api.ListOptions{LabelSelector: selector})); err != nil {
		return nil, err
	} else {
		return podList, nil
	}
}

func (h Handle) CheckContainerExist(c *config.Config) (bool, error) {

	podList, err := h.getPodList(c)

	if err != nil {
		return false, err
	}

	if len(podList.Items) > 0 {
		return false, nil
	}
	return true, nil
}

func (h *Handle) Refresh(c *config.Config) (*api_v1.Pod, error) {
	pod := &api_v1.Pod{}
	podList, err := h.getPodList(c)
	if err != nil {
		return nil, err
	}
	pods := podList.Items
	p := pods[0]
	//
	p.Spec.Containers[0].Image = ""
	return h.client.Pods(c.K8s.Namespace).Update(pod)
}

func (h *Handle) Create(c *config.Config) (*api_v1.Pod, error) {
	p := &api_v1.Pod{}
	p.Kind = "Pod"
	p.Namespace = c.K8s.Namespace
	p.Labels = c.K8s.Labels
	p.APIVersion = "v1"
	p.Name = c.Git.Repository

	container := &api_v1.Container{}
	container.Image = ""
	ports := make([]api_v1.ContainerPort, len(c.Build.Expose))
	for _, expose := range c.Build.Expose {
		port := &api_v1.ContainerPort{}
		port.HostPort = int32(expose)
		port.ContainerPort = int32(expose)
		ports = append(ports, *port)
	}
	containers := make([]api_v1.Container, 1)
	containers = append(containers, *container)
	p.Spec.Containers = containers

	return h.client.Pods(c.K8s.Namespace).Create(p)
}
