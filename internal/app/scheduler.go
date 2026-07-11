package app

import (
	"log"
	"os/exec"
	"strings"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron *cron.Cron
	app  *Server
}

func NewScheduler(app *Server) *Scheduler { return &Scheduler{cron: cron.New(), app: app} }

func (s *Scheduler) Start() {
	for _, task := range s.app.tasks.List() {
		if !task.Enabled || strings.TrimSpace(task.Cron) == "" {
			continue
		}
		t := task
		_, err := s.cron.AddFunc(t.Cron, func() { s.run(t) })
		if err != nil {
			log.Printf("schedule task %s failed: %v", t.Name, err)
		}
	}
	s.cron.Start()
}

func (s *Scheduler) Stop() { s.cron.Stop() }
func (s *Scheduler) run(t ScheduledTask) {
	parts := strings.Fields(t.Action)
	if len(parts) == 0 {
		return
	}
	switch parts[0] {
	case "backup":
		s.app.palworld.Backup()
	case "shell":
		cmd := strings.TrimSpace(strings.TrimPrefix(t.Action, "shell"))
		if cmd != "" {
			_ = exec.Command("sh", "-lc", cmd).Run()
		}
	case "instance":
		if len(parts) >= 3 {
			inst, ok := s.app.instances.Get(parts[2])
			if ok {
				if parts[1] == "start" {
					runInstanceCommand(inst, inst.StartCommand)
				} else if parts[1] == "stop" {
					runInstanceCommand(inst, inst.StopCommand)
				}
			}
		}
	}
}
