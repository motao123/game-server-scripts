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
				switch parts[1] {
				case "start":
					runInstanceCommand(inst, inst.StartCommand)
				case "stop":
					runInstanceCommand(inst, inst.StopCommand)
				case "restart":
					runInstanceCommand(inst, inst.StopCommand)
					runInstanceCommand(inst, inst.StartCommand)
				}
			}
		}
	case "power":
		if len(parts) >= 3 {
			inst, ok := s.app.instances.Get(parts[2])
			if ok {
				switch parts[1] {
				case "start":
					s.app.instances.SetStatus(inst.ID, "running")
				case "stop":
					s.app.instances.SetStatus(inst.ID, "stopped")
				case "restart":
					s.app.instances.SetStatus(inst.ID, "stopped")
					s.app.instances.SetStatus(inst.ID, "running")
				}
			}
		}
	case "command":
		if len(parts) >= 3 {
			inst, ok := s.app.instances.Get(parts[1])
			if ok && inst.Status == "running" {
				cmd := strings.Join(parts[2:], " ")
				runInstanceCommand(inst, cmd)
			}
		}
	case "system":
		if len(parts) >= 2 && parts[1] == "steam_update" {
			log.Println("system task: steam_update (not implemented)")
		}
	}
}

func (s *Scheduler) Reload() {
	s.cron.Stop()
	s.cron = cron.New()
	s.Start()
}
