/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sort"
	"time"

	batchv1 "github.com/cronjob/api/v1"
	"github.com/robfig/cron"
	kbatch "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ref "k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CronJobReconciler reconciles a CronJob object
type CronJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	Clock
}

type realClock struct{}

func (_ realClock) Now() time.Time {
	return time.Now()
}

// clock接口可以获取当前的时间
// 可以帮助我们在测试中模拟计时
type Clock interface {
	Now() time.Time
}

//+kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.tutorial.kubebuilder.io,resources=cronjobs/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CronJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *CronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	//logger = r.Log.WithValues("cronjob", req.NamespacedName)

	// TODO(user): your logic here

	var cronJob batchv1.CronJob
	if err := r.Get(ctx, req.NamespacedName, &cronJob); err != nil {
		logger.Error(err, "unable to fetch CronJob")
		//忽略掉 not-found 错误，它们不能通过重新排队修复（要等待新的通知）
		//在删除一个不存在的对象时，可能会报这个错误。
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var childJobs kbatch.JobList
	if err := r.List(ctx, &childJobs, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name}); err != nil {
		logger.Error(err, "unable to list child Jobs")
		return ctrl.Result{}, err
	}

	// 找出所有有效的 job
	var activeJobs []*kbatch.Job
	var successfulJobs []*kbatch.Job
	var failedJobs []*kbatch.Job
	var mostRecentTime *time.Time // 记录其最近一次运行时间以便更新状态

	//当一个 job 被标记为 “succeeded” 或 “failed” 时，我们认为这个任务处于 “finished” 状态。
	//Status conditions 允许我们给 job 对象添加额外的状态信息，开发人员或控制器可以通过 这些校验信息来检查 job 的完成或健康状态。
	isJobFinished := func(job *kbatch.Job) (bool, kbatch.JobConditionType) {
		for _, c := range job.Status.Conditions {
			if (c.Type == kbatch.JobComplete || c.Type == kbatch.JobFailed) && c.Status == corev1.ConditionTrue {
				return true, c.Type
			}
		}

		return false, ""
	}

	//使用辅助函数来提取创建 job 时注释中排定的执行时间
	getScheduledTimeForJob := func(job *kbatch.Job) (*time.Time, error) {
		timeRaw := job.Annotations[scheduledTimeAnnotation]
		if len(timeRaw) == 0 {
			return nil, nil
		}

		timeParsed, err := time.Parse(time.RFC3339, timeRaw)
		if err != nil {
			return nil, err
		}
		return &timeParsed, nil
	}

	for i, job := range childJobs.Items {
		_, finishedType := isJobFinished(&job)
		switch finishedType {
		case "": // ongoing
			activeJobs = append(activeJobs, &childJobs.Items[i])
		case kbatch.JobFailed:
			failedJobs = append(failedJobs, &childJobs.Items[i])
		case kbatch.JobComplete:
			successfulJobs = append(successfulJobs, &childJobs.Items[i])
		}

		//将启动时间存放在注释中，当job生效时可以从中读取
		scheduledTimeForJob, err := getScheduledTimeForJob(&job)
		if err != nil {
			logger.Error(err, "unable to parse schedule time for child job", "job", &job)
			continue
		}
		if scheduledTimeForJob != nil {
			if mostRecentTime == nil {
				mostRecentTime = scheduledTimeForJob
			} else if mostRecentTime.Before(*scheduledTimeForJob) {
				mostRecentTime = scheduledTimeForJob
			}
		}
	}

	if mostRecentTime != nil {
		cronJob.Status.LastScheduleTime = &metav1.Time{Time: *mostRecentTime}
	} else {
		cronJob.Status.LastScheduleTime = nil
	}
	cronJob.Status.Active = nil
	for _, activeJob := range activeJobs {
		jobRef, err := ref.GetReference(r.Scheme, activeJob)
		if err != nil {
			logger.Error(err, "unable to make reference to active job", "job", activeJob)
			continue
		}
		cronJob.Status.Active = append(cronJob.Status.Active, *jobRef)
	}

	logger.V(1).Info("job count", "active jobs", len(activeJobs), "successful jobs", len(successfulJobs), "failed jobs", len(failedJobs))

	if err := r.Status().Update(ctx, &cronJob); err != nil {
		logger.Error(err, "unable to update CronJob status")
		return ctrl.Result{}, err
	}

	//我们先清理掉一些版本太旧的 job，这样可以不用保留太多无用的 job
	// 注意: 删除操作采用的“尽力而为”策略
	// 如果个别 job 删除失败了，不会将其重新排队，直接结束删除操作
	if cronJob.Spec.FailedJobsHistoryLimit != nil {
		sort.Slice(failedJobs, func(i, j int) bool {
			if failedJobs[i].Status.StartTime == nil {
				return failedJobs[j].Status.StartTime != nil
			}
			return failedJobs[i].Status.StartTime.Before(failedJobs[j].Status.StartTime)
		})
		for i, job := range failedJobs {
			if int32(i) >= int32(len(failedJobs))-*cronJob.Spec.FailedJobsHistoryLimit {
				break
			}
			if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				logger.Error(err, "unable to delete old failed job", "job", job)
			} else {
				logger.V(0).Info("deleted old failed job", "job", job)
			}
		}
	}

	if cronJob.Spec.SuccessfulJobsHistoryLimit != nil {
		sort.Slice(successfulJobs, func(i, j int) bool {
			if successfulJobs[i].Status.StartTime == nil {
				return successfulJobs[j].Status.StartTime != nil
			}
			return successfulJobs[i].Status.StartTime.Before(successfulJobs[j].Status.StartTime)
		})
		for i, job := range successfulJobs {
			if int32(i) >= int32(len(successfulJobs))-*cronJob.Spec.SuccessfulJobsHistoryLimit {
				break
			}
			if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); (err) != nil {
				logger.Error(err, "unable to delete old successful job", "job", job)
			} else {
				logger.V(0).Info("deleted old successful job", "job", job)
			}
		}
	}

	//如果当前 cronjob 被挂起，不会再运行其下的任何 job，我们将其停止。
	//这对于某些 job 出现异常 的排查非常有用。我们无需删除 cronjob 来中止其后续其他 job 的运行。
	if cronJob.Spec.Suspend != nil && *cronJob.Spec.Suspend {
		logger.V(1).Info("cronjob suspended, skipping")
		return ctrl.Result{}, nil
	}

	//如果 cronjob 没被挂起，则我们需要计算它的下一次执行时间， 同时检查是否有遗漏的执行没被处理
	//
	//getNextSchedule
	//
	//借助强大的 cron 库，我们可以轻易的计算出定时任务的下一次执行时间。 我们根据最近一次的执行时间来计算下一次执行时间，如果没有找到最近 一次执行时间，则根据定时任务的创建时间来计算。
	//
	//如果遗漏了很多次执行并且没有为这些执行设置截止时间。我们将其忽略 以避免异常造成控制器的频繁重启和资源紧张。
	//
	//如果遗漏的执行次数并不多，我们返回最近一次的执行时间和下一次将要 执行的时间，这样我们可以知道该何时去调谐。
	getNextSchedule := func(cronJob *batchv1.CronJob, now time.Time) (lastMissed time.Time, next time.Time, err error) {
		sched, err := cron.ParseStandard(cronJob.Spec.Schedule)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("Unparseable schedule %q: %v", cronJob.Spec.Schedule, err)
		}

		// 出于优化的目的，我们可以使用点技巧。从上一次观察到的执行时间开始执行，
		// 这个执行时间可以被在这里被读取。但是意义不大，因为我们刚更新了这个值。

		var earliestTime time.Time
		if cronJob.Status.LastScheduleTime != nil {
			earliestTime = cronJob.Status.LastScheduleTime.Time
		} else {
			earliestTime = cronJob.ObjectMeta.CreationTimestamp.Time
		}
		if cronJob.Spec.StartingDeadlineSeconds != nil {
			// 如果开始执行时间超过了截止时间，不再执行
			schedulingDeadline := now.Add(-time.Second * time.Duration(*cronJob.Spec.StartingDeadlineSeconds))

			if schedulingDeadline.After(earliestTime) {
				earliestTime = schedulingDeadline
			}
		}
		if earliestTime.After(now) {
			return time.Time{}, sched.Next(now), nil
		}

		starts := 0
		for t := sched.Next(earliestTime); !t.After(now); t = sched.Next(t) {
			lastMissed = t
			// 一个 CronJob 可能会遗漏多次执行。举个例子，周五 5:00pm 技术人员下班后，
			// 控制器在 5:01pm 发生了异常。然后直到周二早上才有技术人员发现问题并
			// 重启控制器。那么所有的以1小时为周期执行的定时任务，在没有技术人员
			// 进一步的干预下，都会有 80 多个 job 在恢复正常后一并启动（如果 job 允许
			// 多并发和延迟启动）

			// 如果 CronJob 的某些地方出现异常，控制器或 apiservers (用于设置任务创建时间)
			// 的时钟不正确, 那么就有可能出现错过很多次执行时间的情形（跨度可达数十年）
			// 这将会占满控制器的CPU和内存资源。这种情况下，我们不需要列出错过的全部
			// 执行时间。

			starts++
			if starts > 100 {
				// 获取不到最近一次执行时间，直接返回空切片
				return time.Time{}, time.Time{}, fmt.Errorf("Too many missed start times (> 100). Set or decrease .spec.startingDeadlineSeconds or check clock skew.")
			}
		}
		return lastMissed, sched.Next(now), nil
	}

	// 计算出定时任务下一次执行时间（或是遗漏的执行时间）
	missedRun, nextRun, err := getNextSchedule(&cronJob, r.Now())
	if err != nil {
		logger.Error(err, "unable to figure out CronJob schedule")
		// 重新排队直到有更新修复这次定时任务调度，不必返回错误
		return ctrl.Result{}, nil
	}

	//上述步骤执行完后，将准备好的请求加入队列直到下次执行， 然后确定这些 job 是否要实际执行
	scheduledResult := ctrl.Result{RequeueAfter: nextRun.Sub(r.Now())} // 保存以便别处复用
	logger = logger.WithValues("now", r.Now(), "next run", nextRun)

	//如果 job 遗漏了一次执行，且还没超出截止时间，把遗漏的这次执行也补上
	if missedRun.IsZero() {
		logger.V(1).Info("no upcoming scheduled times, sleeping until next")
		return scheduledResult, nil
	}

	// 确保错过的执行没有超过截止时间
	logger = logger.WithValues("current run", missedRun)
	tooLate := false
	if cronJob.Spec.StartingDeadlineSeconds != nil {
		tooLate = missedRun.Add(time.Duration(*cronJob.Spec.StartingDeadlineSeconds) * time.Second).Before(r.Now())
	}
	if tooLate {
		logger.V(1).Info("missed starting deadline for last run, sleeping till next")
		// TODO(directxman12): events
		return scheduledResult, nil
	}

	//如果确认 job 需要实际执行。我们有三种策略执行该 job。要么先等待现有的 job 执行完后，在启动本次 job；
	//或是直接覆盖取代现有的 job；或是不考虑现有的 job，直接作为新的 job 执行。因为缓存导致的信息有所延迟， 当更新信息后需要重新排队。
	// 确定要 job 的执行策略 —— 并发策略可能禁止多个job同时运行
	if cronJob.Spec.ConcurrencyPolicy == batchv1.ForbidConcurrent && len(activeJobs) > 0 {
		logger.V(1).Info("concurrency policy blocks concurrent runs, skipping", "num active", len(activeJobs))
		return scheduledResult, nil
	}

	// 直接覆盖现有 job
	if cronJob.Spec.ConcurrencyPolicy == batchv1.ReplaceConcurrent {
		for _, activeJob := range activeJobs {
			// we don't care if the job was already deleted
			if err := r.Delete(ctx, activeJob, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				logger.Error(err, "unable to delete active job", "job", activeJob)
				return ctrl.Result{}, err
			}
		}
	}

	//确定如何处理现有 job 后，创建符合我们预期的 job
	// 构建 job

	constructJobForCronJob := func(cronJob *batchv1.CronJob, scheduledTime time.Time) (*kbatch.Job, error) {
		// job 名称带上执行时间以确保唯一性，避免排定执行时间的 job 创建两次
		name := fmt.Sprintf("%s-%d", cronJob.Name, scheduledTime.Unix())

		job := &kbatch.Job{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
				Name:        name,
				Namespace:   cronJob.Namespace,
			},
			Spec: *cronJob.Spec.JobTemplate.Spec.DeepCopy(),
		}
		for k, v := range cronJob.Spec.JobTemplate.Annotations {
			job.Annotations[k] = v
		}
		job.Annotations[scheduledTimeAnnotation] = scheduledTime.Format(time.RFC3339)
		for k, v := range cronJob.Spec.JobTemplate.Labels {
			job.Labels[k] = v
		}
		if err := ctrl.SetControllerReference(cronJob, job, r.Scheme); err != nil {
			return nil, err
		}

		return job, nil
	}

	job, err := constructJobForCronJob(&cronJob, missedRun)
	if err != nil {
		logger.Error(err, "unable to construct job from template")
		// job 的 spec 没有变更，无需重新排队
		return scheduledResult, nil
	}

	// ...在集群中创建 job
	if err := r.Create(ctx, job); err != nil {
		logger.Error(err, "unable to create Job for CronJob", "job", job)
		return ctrl.Result{}, err
	}

	logger.V(1).Info("created Job for CronJob run", "job", job)

	// 当有 job 进入运行状态后，重新排队，同时更新状态

	return ctrl.Result{}, nil
}

var (
	scheduledTimeAnnotation = "batch.tutorial.kubebuilder.io/scheduled-at"
)

var (
	jobOwnerKey = ".metadata.controller"
	apiGVStr    = batchv1.GroupVersion.String()
)

// SetupWithManager sets up the controller with the Manager.
func (r *CronJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// 此处不是测试，我们需要创建一个真实的时钟
	if r.Clock == nil {
		r.Clock = realClock{}
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &kbatch.Job{}, jobOwnerKey, func(rawObj client.Object) []string {
		//获取 job 对象，提取 owner...
		job := rawObj.(*kbatch.Job)
		owner := metav1.GetControllerOf(job)
		if owner == nil {
			return nil
		}
		// ...确保 owner 是个 CronJob...
		if owner.APIVersion != apiGVStr || owner.Kind != "CronJob" {
			return nil
		}

		// ...是 CronJob，返回
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.CronJob{}).
		Complete(r)
}
