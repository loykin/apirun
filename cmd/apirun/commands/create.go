package commands

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/loykin/apirun"
	"github.com/loykin/apirun/cmd/apirun/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var CreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new migration file with a task template (timestamp-based name)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v := viper.GetViper()
		configPath := v.GetString("config")

		// Determine migration directory similar to status command
		dir := ""
		if strings.TrimSpace(configPath) != "" {
			var doc config.ConfigDoc
			if err := doc.Load(configPath); err != nil {
				log.Printf("warning: failed to load config: %v", err)
			} else {
				mDir := strings.TrimSpace(doc.MigrateDir)
				if mDir == "" {
					mDir = filepath.Dir(configPath)
				}
				dir = mDir
			}
		}
		if strings.TrimSpace(dir) == "" {
			// Default CLI location used by README and other commands
			dir = "./config/migration"
		}

		name := "task"
		if len(args) > 0 {
			name = args[0]
		}

		p, err := apirun.CreateMigration(apirun.CreateOptions{Name: name, Dir: dir})
		if err != nil {
			return err
		}
		fmt.Println(p)
		return nil
	},
}
