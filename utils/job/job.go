package job

import (
	"context"
	"fmt"
	"io"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/zhujintao/kit-go/utils/job/progress"
)

type Jobs struct {
	name      string
	added     map[string]struct{}
	mu        sync.Mutex
	descs     []string
	resolved  bool
	active    []StatusInfo
	activeMap map[string]StatusInfo
	ctx       context.Context
	done      context.CancelFunc
	activeCh  chan StatusInfo
}

func (j *Jobs) SendStatus(s StatusInfo) {
	j.activeCh <- s
}

func (j *Jobs) StartProcess(ongoing *Jobs, out io.Writer) {

	var (
		fw       = progress.NewWriter(out)
		start    = time.Now()
		statuses = map[string]StatusInfo{}
		done     bool
	)

	for {

		select {

		case active := <-j.activeCh:

			fw.Flush()
			tw := tabwriter.NewWriter(fw, 1, 8, 1, ' ', 0)

			resolved := StatusResolved
			if !ongoing.IsResolved() {
				resolved = StatusResolving
			}
			statuses[ongoing.name] = StatusInfo{
				Ref:    ongoing.name,
				Status: resolved,
			}
			keys := []string{ongoing.name}

			activeSeen := map[string]struct{}{}
			if !done {
				//active, err := cs.ListStatuses(ctx, "")
				//if err != nil {
				//	log.G(ctx).WithError(err).Error("active check failed")
				//	continue
				//}
				// update status of active entries!

				statuses[active.Ref] = StatusInfo{
					Ref:       active.Ref,
					Status:    active.Status,
					Offset:    active.Offset,
					Total:     active.Total,
					StartedAt: active.StartedAt,
					UpdatedAt: active.UpdatedAt,
				}

				activeSeen[active.Ref] = struct{}{}

			}

			for _, job := range ongoing.Jobs() {
				//	fmt.Println("-----------")
				//key := remotes.MakeRefKey(ctx, j)
				key := job
				keys = append(keys, key)
				if _, ok := activeSeen[key]; ok {
					continue
				}

				status, ok := statuses[key]

				if !done && (!ok || status.Status == StatusRunning) {

					/*
						info, err := cs.Info(ctx, j.Digest)
						if err != nil {
							if !errdefs.IsNotFound(err) {
								log.G(ctx).WithError(err).Error("failed to get content info")
								continue outer
							} else {
								statuses[key] = StatusInfo{
									Ref:    key,
									Status: StatusWaiting,
								}
							}
						} else if info.CreatedAt.After(start) {
							statuses[key] = StatusInfo{
								Ref:       key,
								Status:    StatusDone,
								Offset:    info.Size,
								Total:     info.Size,
								UpdatedAt: info.CreatedAt,
							}
						} else {
							statuses[key] = StatusInfo{
								Ref:    key,
								Status: StatusExists,
							}
						}
					*/

				} else if done {
					if ok {
						if status.Status != StatusDone && status.Status != StatusExists {
							status.Status = StatusDone
							statuses[key] = status

						}
					} else {
						statuses[key] = StatusInfo{
							Ref:    key,
							Status: StatusDone,
						}
					}
				}

			}

			var ordered []StatusInfo
			for _, key := range keys {
				ordered = append(ordered, statuses[key])
			}

			Display(tw, ordered, start)
			tw.Flush()

			if done {
				fw.Flush()
				close(j.activeCh)
				return
			}
		case <-j.ctx.Done():

			done = true

		}
	}

}

func NewJobs(name string) *Jobs {
	ctx, done := context.WithCancel(context.Background())
	return &Jobs{
		name:     name,
		added:    map[string]struct{}{},
		ctx:      ctx,
		done:     done,
		activeCh: make(chan StatusInfo),
	}
}
func (j *Jobs) SetStatus(active ...StatusInfo) {
	//	j.active = []StatusInfo{}
	j.active = append(j.active, active...)

}
func (j *Jobs) Add(desc string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.resolved = true

	if _, ok := j.added[desc]; ok {
		return
	}

	j.descs = append(j.descs, desc)
	j.added[desc] = struct{}{}

}

func (j *Jobs) Jobs() []string {
	j.mu.Lock()
	defer j.mu.Unlock()

	var descs []string

	return append(descs, j.descs...)
}

func (j *Jobs) IsResolved() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.resolved
}

type statusInfoStatus string

const (
	StatusDone      statusInfoStatus = "done"
	StatusWaiting   statusInfoStatus = "waiting"
	StatusExists    statusInfoStatus = "exists"
	StatusRunning   statusInfoStatus = "running"
	StatusResolved  statusInfoStatus = "resolved"
	StatusResolving statusInfoStatus = "resolving"
)

type StatusInfo struct {
	Ref       string
	Status    statusInfoStatus
	Offset    int64
	Total     int64
	StartedAt time.Time
	UpdatedAt time.Time
}

func Display(w io.Writer, statuses []StatusInfo, start time.Time) {

	var total int64
	for _, status := range statuses {

		total += status.Offset

		switch status.Status {

		case StatusRunning:
			var bar progress.Bar
			if status.Total > 0.0 {
				bar = progress.Bar(float64(status.Offset) / float64(status.Total))

			}
			/*
				fmt.Fprintf(w, "%s:\t%s\t%40r\t%8.8s/%s\t\n",
					status.Ref,
					status.Status,
					bar,

					progress.Bytes(status.Offset), progress.Bytes(status.Total),
				)
			*/
			fmt.Fprintf(w, "%s:\t%s\t%40r\t%d/%d\t\n",
				status.Ref,
				status.Status,
				bar,

				status.Offset, status.Total,
			)

		case StatusWaiting:
			bar := progress.Bar(0.0)
			fmt.Fprintf(w, "%s:\t%s\t%40r\t\n",
				status.Ref,
				status.Status,
				bar)
		default:
			bar := progress.Bar(1.0)
			fmt.Fprintf(w, "%s:\t%s\t%40r\t\n",
				status.Ref,
				status.Status,
				bar)

		}

	}
	fmt.Fprintf(w, "elapsed: %-4.1fs\ttotal: %7.6v\t(%v)\t\n",
		time.Since(start).Seconds(),

		total,
		int64(float64(total)/time.Since(start).Seconds()))
}
func (j *Jobs) Done() {
	time.Sleep(time.Millisecond * 200)
	j.done()
}
func (j *Jobs) ShowProcess(ongoing *Jobs, out io.Writer) {

	var (
		ticker   = time.NewTicker(100 * time.Millisecond)
		fw       = progress.NewWriter(out)
		start    = time.Now()
		statuses = map[string]StatusInfo{}
		done     bool
	)
	defer ticker.Stop()

	//outer:

	for {
		select {
		case <-ticker.C:

			//	fmt.Println(len(j.active))
			fw.Flush()

			tw := tabwriter.NewWriter(fw, 1, 8, 1, ' ', 0)

			resolved := StatusResolved
			if !ongoing.IsResolved() {
				resolved = StatusResolving
			}
			statuses[ongoing.name] = StatusInfo{
				Ref:    ongoing.name,
				Status: resolved,
			}
			keys := []string{ongoing.name}

			activeSeen := map[string]struct{}{}
			if !done {
				//active, err := cs.ListStatuses(ctx, "")
				//if err != nil {
				//	log.G(ctx).WithError(err).Error("active check failed")
				//	continue
				//}
				// update status of active entries!

				for _, active := range j.active {

					statuses[active.Ref] = StatusInfo{
						Ref:       active.Ref,
						Status:    active.Status,
						Offset:    active.Offset,
						Total:     active.Total,
						StartedAt: active.StartedAt,
						UpdatedAt: active.UpdatedAt,
					}

					activeSeen[active.Ref] = struct{}{}

				}
			}

			// now, update the items in jobs that are not in active
			for _, job := range ongoing.Jobs() {
				//	fmt.Println("-----------")
				//key := remotes.MakeRefKey(ctx, j)
				key := job
				keys = append(keys, key)
				if _, ok := activeSeen[key]; ok {
					continue
				}

				status, ok := statuses[key]

				if !done && (!ok || status.Status == StatusRunning) {

					/*
						info, err := cs.Info(ctx, j.Digest)
						if err != nil {
							if !errdefs.IsNotFound(err) {
								log.G(ctx).WithError(err).Error("failed to get content info")
								continue outer
							} else {
								statuses[key] = StatusInfo{
									Ref:    key,
									Status: StatusWaiting,
								}
							}
						} else if info.CreatedAt.After(start) {
							statuses[key] = StatusInfo{
								Ref:       key,
								Status:    StatusDone,
								Offset:    info.Size,
								Total:     info.Size,
								UpdatedAt: info.CreatedAt,
							}
						} else {
							statuses[key] = StatusInfo{
								Ref:    key,
								Status: StatusExists,
							}
						}
					*/

				} else if done {
					if ok {
						if status.Status != StatusDone && status.Status != StatusExists {
							status.Status = StatusDone
							statuses[key] = status
						}
					} else {
						statuses[key] = StatusInfo{
							Ref:    key,
							Status: StatusDone,
						}
					}
				}

			}

			var ordered []StatusInfo
			for _, key := range keys {
				ordered = append(ordered, statuses[key])
			}

			Display(tw, ordered, start)
			tw.Flush()

			if done {
				fw.Flush()
				return
			}
		case <-j.ctx.Done():

			done = true // allow ui to update once more
		}
	}
}
