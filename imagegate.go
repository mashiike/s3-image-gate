package imagegate

import (
	"context"
	"fmt"
	"log"

	"github.com/fujiwara/ridge"
)

func Run(ctx context.Context, cfg *Config) error {
	handler, err := cfg.NewHander()
	if err != nil {
		return err
	}
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("[info] s3-image-gate starting up on %s", addr)
	ridge.RunWithContext(ctx, addr, "/", handler)
	return nil
}
