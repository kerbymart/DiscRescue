package devtool

import (
	"context"
	"fmt"
)

func (a App) runNamed(ctx context.Context, name string, command Command) error {
	fmt.Fprintf(a.Out, "[devtool] %s: running\n", name)
	if err := a.Runner.Run(ctx, command); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}
