package rc

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/airbusgeo/geocube/internal/log"

	"go.uber.org/zap"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const PMLABEL = "go-k8s-pod-rc"

type ReplicationController struct {
	client              *kubernetes.Clientset
	rcname, rcnamespace string
	CostLimit           int    //terminationcost over which a pod will not be deleted
	CostPort            int    //port on which to poll for pod termination cost
	CostPath            string //(http)path on which to poll for pod termination cost
	AllowEviction       bool   //can a created pod be terminated by the cluster autoscaler
	//SpreadOnNodes true disables the default behavior to try to schedule pods alongside existing ones.
	//The default behavior is to prefer packing nodes with pods in order to more rapidly drain nodes so
	//they can be shut down by the cluster autoscaler. This is useful in case there is a 1-to-1 ratio of
	//pods vs. jobs to execute, but is detrimental in case there are fewer pods than jobs.
	SpreadOnNodes bool
	httpcl        http.Client
}

func New(name, namespace string) (*ReplicationController, error) {
	d := ReplicationController{
		rcname:      name,
		rcnamespace: namespace,
	}
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("rest.inclusterconfig. Is code running on k8s?: %w", err)
	}
	// creates the clientset
	d.client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("kubernetes.newforconfig: %w", err)
	}
	d.httpcl.Timeout = 3 * time.Second
	return &d, nil
}

func isPodActive(p apiv1.Pod) bool {
	return apiv1.PodSucceeded != p.Status.Phase &&
		apiv1.PodFailed != p.Status.Phase &&
		p.DeletionTimestamp == nil
}

func (d *ReplicationController) listPods(ctx context.Context, namespace, name string) ([]apiv1.Pod, error) {
	opts := metav1.ListOptions{}
	lbls := make(map[string]string)
	lbls[PMLABEL] = name
	opts.LabelSelector = labels.Set(lbls).AsSelector().String()
	pods, err := d.client.CoreV1().Pods(namespace).List(opts)
	if err != nil {
		return nil, fmt.Errorf("list (%s) pods : %w", name, err)
	}
	return pods.Items, nil
}
func (d *ReplicationController) listActivePods(ctx context.Context, namespace, name string) ([]apiv1.Pod, error) {
	pods, err := d.listPods(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	activePods := []apiv1.Pod{}
	for _, pod := range pods {
		if isPodActive(pod) {
			activePods = append(activePods, pod)
		}
	}
	return activePods, nil
}

func (d *ReplicationController) createPod(ctx context.Context, namespace, name string) (apiv1.Pod, error) {

	depl, err := d.client.CoreV1().ReplicationControllers(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return apiv1.Pod{}, fmt.Errorf("get replication controller: %w", err)
	}
	pod := apiv1.Pod{}
	pod.Spec = depl.Spec.Template.Spec
	pod.GenerateName = name + "-" + PMLABEL + "-"
	if len(pod.Labels) == 0 {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[PMLABEL] = name
	if len(pod.Annotations) == 0 {
		pod.Annotations = make(map[string]string)
	}
	if d.AllowEviction {
		pod.Annotations["cluster-autoscaler.kubernetes.io/safe-to-evict"] = "true"
	}
	if !d.SpreadOnNodes {
		if pod.Spec.Affinity == nil {
			pod.Spec.Affinity = &apiv1.Affinity{}
		}
		if pod.Spec.Affinity.PodAffinity == nil {
			pod.Spec.Affinity.PodAffinity = &apiv1.PodAffinity{}
		}
		pod.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution = append(
			pod.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution,
			apiv1.WeightedPodAffinityTerm{
				Weight: 1,
				PodAffinityTerm: apiv1.PodAffinityTerm{
					TopologyKey: "kubernetes.io/hostname",
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      PMLABEL,
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{name},
							},
						},
					},
				},
			})
	}

	return pod, nil
}

func (d *ReplicationController) Size(ctx context.Context) (int64, error) {
	pods, err := d.listActivePods(ctx, d.rcnamespace, d.rcname)
	if err != nil {
		return 0, fmt.Errorf("listPods: %w", err)
	}
	return int64(len(pods)), nil
}

func (d *ReplicationController) Resize(ctx context.Context, newSize int64) error {

	allpods, err := d.listPods(ctx, d.rcnamespace, d.rcname)
	if err != nil {
		return fmt.Errorf("listPods: %w", err)
	}

	if newSize == 0 {
		for _, pod := range allpods {
			del := &metav1.DeleteOptions{
				GracePeriodSeconds: new(int64), //0
			}
			err = d.client.CoreV1().Pods(d.rcnamespace).Delete(pod.Name, del)
			if err != nil {
				return fmt.Errorf("delete pod: %w", err)
			}
		}
		return nil
	}

	active := []apiv1.Pod{}
	inactive := []apiv1.Pod{}
	for _, pod := range allpods {
		if isPodActive(pod) {
			active = append(active, pod)
		} else {
			inactive = append(inactive, pod)
		}
	}
	for _, pod := range inactive {
		del := &metav1.DeleteOptions{
			GracePeriodSeconds: new(int64), //0
		}
		log.Logger(ctx).Sugar().Infof("delete %s pod %s", pod.Status.Phase, pod.Name)
		err = d.client.CoreV1().Pods(d.rcnamespace).Delete(pod.Name, del)
		if err != nil {
			log.Logger(ctx).Error("delete "+pod.Name, zap.Error(err))
		}
	}

	if int(newSize) > len(active) {
		for _, pod := range active {
			if pod.Status.Phase == apiv1.PodPending {
				return fmt.Errorf("not applying resize to %d, cluster not at target", newSize)
			}
		}
		for i := int64(len(active)); i < newSize; i++ {
			pod, err := d.createPod(ctx, d.rcnamespace, d.rcname)
			if err != nil {
				return fmt.Errorf("createPod: %w", err)
			}
			_, err = d.client.CoreV1().Pods(d.rcnamespace).Create(&pod)
			if err != nil {
				return fmt.Errorf("createPod: %w", err)
			}
		}
	}

	if int(newSize) < len(active) {
		for i := int(newSize); i < len(active); i++ {
			del := &metav1.DeleteOptions{
				GracePeriodSeconds: new(int64), //0
			}
			//log.Printf("delete pod %s", pods[i].Name)
			err = d.client.CoreV1().Pods(d.rcnamespace).Delete(active[i].Name, del)
			if err != nil {
				return fmt.Errorf("deletePod: %w", err)
			}
		}
	}
	return nil
}

func (d *ReplicationController) getPodTerminationCost(ctx context.Context, p apiv1.Pod) (int, error) {
	if d.CostPath == "" {
		return 0, nil
	}
	u := "http://" + p.Status.PodIP
	if d.CostPort != 0 && d.CostPort != 80 {
		u += fmt.Sprintf(":%d", d.CostPort)
	}
	if !strings.HasPrefix(d.CostPath, "/") {
		u += "/" + d.CostPath
	} else {
		u += d.CostPath
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return 0, fmt.Errorf("new request %s: %w", u, err)
	}
	req = req.WithContext(ctx)

	resp, err := d.httpcl.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http.do %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("cost on %s returned code %d: %w", u, resp.StatusCode, err)
	}
	var cost int
	_, err = fmt.Fscan(resp.Body, &cost)
	if err != nil {
		return 0, fmt.Errorf("scan cost from body: %w", err)
	}
	return cost, nil
}

// IsPodReady returns true if a pod is ready; false otherwise.
func IsPodReadyConditionTrue(status apiv1.PodStatus) bool {
	condition := GetPodReadyCondition(status)
	return condition != nil && condition.Status == apiv1.ConditionTrue
}

// Extracts the pod ready condition from the given status and returns that.
// Returns nil if the condition is not present.
func GetPodReadyCondition(status apiv1.PodStatus) *apiv1.PodCondition {
	_, condition := GetPodCondition(&status, apiv1.PodReady)
	return condition
}

// GetPodCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func GetPodCondition(status *apiv1.PodStatus, conditionType apiv1.PodConditionType) (int, *apiv1.PodCondition) {
	if status == nil {
		return -1, nil
	}
	return GetPodConditionFromList(status.Conditions, conditionType)
}

// GetPodConditionFromList extracts the provided condition from the given list of condition and
// returns the index of the condition and the condition. Returns -1 and nil if the condition is not present.
func GetPodConditionFromList(conditions []apiv1.PodCondition, conditionType apiv1.PodConditionType) (int, *apiv1.PodCondition) {
	if conditions == nil {
		return -1, nil
	}
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return i, &conditions[i]
		}
	}
	return -1, nil
}

type podCost struct {
	idx  int
	cost int
	node string
}

type costs []podCost

func (c costs) Len() int      { return len(c) }
func (c costs) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c costs) Less(i, j int) bool {
	return c[i].cost < c[j].cost
}

type nc struct {
	node  string
	count int
}
type ncs []nc

func (n ncs) Len() int           { return len(n) }
func (n ncs) Swap(i, j int)      { n[i], n[j] = n[j], n[i] }
func (n ncs) Less(i, j int) bool { return n[i].count < n[j].count }

type podsOfNode struct {
	count      int
	candidates []podCost
}

//return needed pods in best order of candidacy for deletion
func (d *ReplicationController) orderedPodsCandidateForDeletion(ctx context.Context, pods []apiv1.Pod, needed int) ([]apiv1.Pod, error) {
	type PodLog struct {
		Pod  string
		Cost int
	}
	NodeLog := make(map[string][]PodLog)

	podsPerNode := make(map[string]podsOfNode) //number of pods per node that will be left after deletion of the needed number of pods
	ret := []apiv1.Pod{}
	tomonitor := []apiv1.Pod{}
	for _, p := range pods {
		if p.Status.Phase != apiv1.PodRunning || !IsPodReadyConditionTrue(p.Status) {
			ret = append(ret, p)
			NodeLog[p.Spec.NodeName] = append(NodeLog[p.Spec.NodeName], PodLog{p.Name, -1})
		} else {
			pon := podsPerNode[p.Spec.NodeName]
			pon.count++
			podsPerNode[p.Spec.NodeName] = pon
			tomonitor = append(tomonitor, p)
		}
	}
	if len(ret) >= needed {
		return ret[0:needed], nil
	}
	stillneeded := needed - len(ret)

	costs := costs{}
	wg := sync.WaitGroup{}
	mu := sync.Mutex{}

	for i, p := range tomonitor {
		wg.Add(1)
		go func(i int, p apiv1.Pod) {
			defer wg.Done()
			var err error
			cost := podCost{idx: i, node: p.Spec.NodeName}
			cost.cost, err = d.getPodTerminationCost(ctx, p)
			if err != nil {
				log.Logger(ctx).Debug("failed to get pod termination cost", zap.Error(err))
			} else {
				mu.Lock()
				costs = append(costs, cost)
				mu.Unlock()
			}
		}(i, p)

	}
	wg.Wait()
	sort.Sort(costs)

	//we cannot delete pods who's cost is over this value
	costCuttOff := 0
	if len(costs) >= stillneeded {
		costCuttOff = costs[stillneeded-1].cost
	} else {
		costCuttOff = costs[len(costs)-1].cost
	}
	if d.CostLimit > 0 && costCuttOff > d.CostLimit {
		costCuttOff = d.CostLimit
	}

	//update number of pods per node if we were to remove all pods who's cost make them candidate for deletion
	for _, c := range costs {
		if c.cost <= costCuttOff {
			pon := podsPerNode[c.node]
			pon.count--
			pon.candidates = append(pon.candidates, c)
			podsPerNode[c.node] = pon
		}
	}

	//sort nodes by number of pods remaining after deletion
	nodeCounts := ncs{}
	for n, c := range podsPerNode {
		nodeCounts = append(nodeCounts, nc{n, c.count})
	}
	sort.Sort(nodeCounts)

outer:
	for _, nc := range nodeCounts {
		for _, pc := range podsPerNode[nc.node].candidates {
			ret = append(ret, pods[pc.idx])
			NodeLog[nc.node] = append(NodeLog[nc.node], PodLog{pods[pc.idx].Name, pc.cost})
			if len(ret) == needed {
				break outer
			}
		}
	}

	log.Logger(ctx).Sugar().With(zap.Any("nodes", NodeLog)).Debugf("%d pods candidate for deletion", len(ret))
	return ret, nil
}

func (d *ReplicationController) ScaleDown(ctx context.Context, newSize int64) error {
	pods, err := d.listPods(ctx, d.rcnamespace, d.rcname)
	if err != nil {
		return fmt.Errorf("listPods: %w", err)
	}
	delta := len(pods) - int(newSize)
	if delta <= 0 {
		return fmt.Errorf("scaledown called with invalid size %d (%d replicas available)", newSize, len(pods))
	}
	pd, err := d.orderedPodsCandidateForDeletion(ctx, pods, delta)
	for _, p := range pd {
		del := &metav1.DeleteOptions{
			GracePeriodSeconds: new(int64), //0
		}
		//log.Printf("scale down pods %s", p.Name)
		log.Logger(ctx).Sugar().Debugf("scaling down pod %s", p.Name)
		err = d.client.CoreV1().Pods(d.rcnamespace).Delete(p.Name, del)
		if err != nil {
			return fmt.Errorf("delete idle pod %s: %w", p.Name, err)
		}
	}
	return nil
}
