package devtool

import "context"

func (a App) runCheck(ctx context.Context) error {
	steps := []struct {
		name string
		args []string
	}{
		{"format", []string{"format", "--check"}},
		{"vet", []string{"vet", "./..."}},
		{"test", []string{"test", "./..."}},
		{"build", []string{"build", "-trimpath", "./cmd/discrescue"}},
	}
	for _, step := range steps {
		if step.args[0] == "format" {
			if err := a.runFormat(ctx, step.args[1:]); err != nil {
				return err
			}
			continue
		}
		if err := a.runNamed(ctx, step.name, Command{Name: "go", Args: step.args, Dir: a.Root, Env: defaultEnv()}); err != nil {
			return err
		}
	}
	return nil
}
