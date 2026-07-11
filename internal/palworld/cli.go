package palworld

import (
	"fmt"
	"os"

	"game-server-scripts/internal/config"
)

func RunCLI(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing palworld command")
	}
	s := Service{Config: config.Load()}
	if args[0] == "manager" {
		return runManager(s, args[1:])
	}
	return fmt.Errorf("unknown palworld command %q", args[0])
}

func runManager(s Service, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing manager command")
	}
	switch args[0] {
	case "start", "stop", "restart":
		out, err := s.Systemctl(args[0])
		fmt.Print(out)
		return err
	case "status":
		st := s.Status()
		fmt.Printf("active=%v uptime=%s\n", st.Active, st.Uptime)
	case "backup":
		ok, msg := s.Backup()
		fmt.Print(msg)
		if !ok {
			os.Exit(1)
		}
	case "players":
		for _, p := range s.Players() {
			fmt.Printf("%s %s %s\n", p.Name, p.PlayerUID, p.SteamID)
		}
	case "broadcast":
		if len(args) < 2 {
			return fmt.Errorf("message required")
		}
		out, err := s.RCON("Broadcast " + args[1])
		fmt.Print(out)
		return err
	case "save":
		out, err := s.RCON("Save")
		fmt.Print(out)
		return err
	default:
		return fmt.Errorf("unknown manager command %q", args[0])
	}
	return nil
}
