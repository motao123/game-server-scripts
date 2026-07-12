package app

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
			s.app.tasks.SetNextRun(task.ID, "")
			continue
		}
		t := task
		_, err := s.cron.AddFunc(t.Cron, func() { s.RunTask(t.ID) })
		if err != nil {
			log.Printf("schedule task %s failed: %v", t.Name, err)
			continue
		}
		s.app.tasks.SetNextRun(t.ID, nextRunFor(t.Cron))
	}
	s.cron.Start()
}

func (s *Scheduler) Stop() { s.cron.Stop() }

func (s *Scheduler) Reload() {
	s.cron.Stop()
	s.cron = cron.New()
	s.Start()
}

func (s *Scheduler) RunTask(id string) {
	task, ok := s.app.tasks.Get(id)
	if !ok {
		return
	}
	err := runWithHistory(s.app.taskHistory, id, func() error {
		return s.run(task)
	})
	s.app.tasks.SetRunResult(id, err)
	s.RefreshNextRun(id)
	if err != nil {
		log.Printf("schedule task %s failed: %v", task.Name, err)
	}
}

func (s *Scheduler) RefreshNextRun(id string) {
	task, ok := s.app.tasks.Get(id)
	if !ok || !task.Enabled || strings.TrimSpace(task.Cron) == "" {
		s.app.tasks.SetNextRun(id, "")
		return
	}
	s.app.tasks.SetNextRun(id, nextRunFor(task.Cron))
}

func nextRunFor(expr string) string {
	schedule, err := cron.ParseStandard(expr)
	if err != nil {
		return ""
	}
	return schedule.Next(time.Now()).Format(time.RFC3339)
}

func (s *Scheduler) run(t ScheduledTask) error {
	parts := strings.Fields(t.Action)
	if len(parts) == 0 {
		return fmt.Errorf("任务动作为空")
	}
	switch parts[0] {
	case "backup":
		if t.BackupSourcePath != "" {
			name := t.BackupName
			if name == "" {
				name = filepath.Base(t.BackupSourcePath)
			}
			_, err := s.app.backups.Create(name, t.BackupSourcePath, t.MaxKeep)
			if err != nil {
				return fmt.Errorf("备份失败: %w", err)
			}
			return nil
		}
		ok, msg := s.app.palworld.Backup()
		if !ok {
			return fmt.Errorf(msg)
		}
	case "shell":
		cmd := strings.TrimSpace(strings.TrimPrefix(t.Action, "shell"))
		if cmd == "" {
			return fmt.Errorf("shell 命令为空")
		}
		out, err := exec.Command("sh", "-lc", cmd).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
		}
	case "instance", "power":
		if len(parts) < 3 {
			return fmt.Errorf("实例任务参数不足")
		}
		return s.runInstanceAction(parts[1], parts[2])
	case "command":
		if len(parts) < 3 {
			return fmt.Errorf("命令任务参数不足")
		}
		inst, ok := s.app.instances.Get(parts[1])
		if !ok {
			return fmt.Errorf("实例不存在")
		}
		if inst.Status != "running" {
			return fmt.Errorf("实例未运行")
		}
		cmd := strings.Join(parts[2:], " ")
		out, err := runInstanceCommand(inst, cmd)
		if err != nil {
			return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
		}
	case "system":
		if len(parts) >= 2 && parts[1] == "steam_update" {
			log.Println("system task: steam_update (not implemented)")
			return nil
		}
		return fmt.Errorf("未知系统任务")
	default:
		return fmt.Errorf("未知任务类型: %s", parts[0])
	}
	return nil
}

func (s *Scheduler) runInstanceAction(action, id string) error {
	inst, ok := s.app.instances.Get(id)
	if !ok {
		return fmt.Errorf("实例不存在")
	}
	switch action {
	case "start":
		_, err := s.app.runtime.Start(inst)
		return err
	case "stop":
		s.app.runtime.Stop(inst)
		return nil
	case "restart":
		s.app.runtime.Stop(inst)
		updated, ok := s.app.instances.Get(id)
		if !ok {
			return fmt.Errorf("实例不存在")
		}
		_, err := s.app.runtime.Start(updated)
		return err
	default:
		return fmt.Errorf("未知实例动作: %s", action)
	}
}
