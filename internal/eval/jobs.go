package eval

import (
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Job tracks a background job.
type Job struct {
	ID       int
	Pgid     int
	Pids     []int
	Cmd      string
	State    string
	Exit     int
	Notified bool
	Done     chan int
}

func (r *Runner) addJob(pgid int, pids []int, cmd string) *Job {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Jobs == nil {
		r.Jobs = make(map[int]*Job)
	}
	r.nextJobID++
	job := &Job{
		ID:    r.nextJobID,
		Pgid:  pgid,
		Pids:  append([]int{}, pids...),
		Cmd:   cmd,
		State: "running",
		Done:  make(chan int, 1),
	}
	r.Jobs[r.nextJobID] = job
	return job
}

func (r *Runner) markJobDone(pgid int, exit int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, job := range r.Jobs {
		if job.Pgid == pgid {
			job.State = "done"
			job.Exit = exit
			job.Notified = false
			return
		}
	}
}

func (r *Runner) findJobByPgid(pgid int) *Job {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, job := range r.Jobs {
		if job.Pgid == pgid {
			return job
		}
	}
	return nil
}

func (r *Runner) findJobByPID(pid int) *Job {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, job := range r.Jobs {
		if job.Pgid == pid {
			return job
		}
		for _, p := range job.Pids {
			if p == pid {
				return job
			}
		}
	}
	return nil
}

func (r *Runner) waitJob(job *Job) int {
	if job == nil {
		return 1
	}
	if job.State == "done" {
		return job.Exit
	}
	exit, ok := <-job.Done
	if ok {
		return exit
	}
	return job.Exit
}

func (r *Runner) removeJob(id int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Jobs, id)
}

func (r *Runner) listJobs() []*Job {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	jobs := make([]*Job, 0, len(r.Jobs))
	for _, job := range r.Jobs {
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].ID < jobs[j].ID
	})
	return jobs
}

func (r *Runner) pruneJobs() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, job := range r.Jobs {
		if job.State == "done" && job.Notified {
			delete(r.Jobs, id)
		}
	}
}

func (r *Runner) getJob(id int) *Job {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Jobs[id]
}

func (r *Runner) lastJob() *Job {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var last *Job
	for _, job := range r.Jobs {
		if last == nil || job.ID > last.ID {
			last = job
		}
	}
	return last
}

func (r *Runner) addAPID(pid int) {
	if r == nil || r.Env == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	vals := append([]string{}, r.Env.Get("apid")...)
	vals = append(vals, strconv.Itoa(pid))
	r.Env.Set("apid", vals)
}

func (r *Runner) removeAPID(pid int) {
	if r == nil || r.Env == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	val := strconv.Itoa(pid)
	vals := r.Env.Get("apid")
	if len(vals) == 0 {
		return
	}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if v != val {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		r.Env.Unset("apid")
		return
	}
	r.Env.Set("apid", out)
}

func formatJobs(jobs []*Job) string {
	var b strings.Builder
	for _, job := range jobs {
		b.WriteString("[")
		b.WriteString(strconv.Itoa(job.ID))
		b.WriteString("] ")
		b.WriteString(job.State)
		b.WriteString(" ")
		b.WriteString(strconv.Itoa(job.Pgid))
		b.WriteString(" ")
		b.WriteString(job.Cmd)
		b.WriteString("\n")
	}
	return b.String()
}

var _ = sync.Mutex{}
