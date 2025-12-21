package eval

import (
	"strconv"
	"strings"
	"sync"
)

// Job tracks a background job.
type Job struct {
	ID    int
	Pgid  int
	Pids  []int
	Cmd   string
	State string
	Exit  int
}

func (r *Runner) addJob(pgid int, pid int, cmd string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.Jobs == nil {
		r.Jobs = make(map[int]*Job)
	}
	r.nextJobID++
	r.Jobs[r.nextJobID] = &Job{ID: r.nextJobID, Pgid: pgid, Pids: []int{pid}, Cmd: cmd, State: "running"}
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
			return
		}
	}
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
	return jobs
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
