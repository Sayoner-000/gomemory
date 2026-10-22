package usecases

import (
	"context"
	"mem/application/ports"
	"mem/domain"
	"time"
)

type ShieldScanner struct {
	Reader ports.AgentConfigReaderPort
	State  ports.ShieldStateRepository
	Now    func() time.Time
}

func (s ShieldScanner) scan(ctx context.Context, hash string) (domain.ShieldReport, error) {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	report := domain.ShieldReport{Hash: hash, ScannedAt: now(), Findings: []domain.ShieldFinding{}, Skipped: []string{}}
	if s.Reader == nil {
		return report, nil
	}
	files, skipped := s.Reader.Files(ctx, 32, 1<<20)
	report.Skipped = skipped
	for _, f := range files {
		if ctx.Err() != nil {
			report.Skipped = append(report.Skipped, f.Path)
			continue
		}
		found := domain.ScanAgentConfig(f.Path, f.Content, f.Home, f.PathDirs, f.UnsafePaths)
		if f.Rewritten {
			// Una línea del texto transformado señalaría un sitio falso.
			for i := range found {
				found[i].Line = 0
			}
		}
		report.Findings = append(report.Findings, found...)
	}
	report = report.Sanitized()
	if s.State != nil {
		if err := s.State.Save(ctx, report); err != nil {
			return report, err
		}
	}
	return report, ctx.Err()
}

func (s ShieldScanner) Scan(ctx context.Context, _ domain.ExpandSignals) (domain.ShieldReport, bool, error) {
	if s.Reader == nil {
		return domain.ShieldReport{}, false, nil
	}
	hash := s.Reader.Fingerprint(ctx)
	if hash == "" {
		return domain.ShieldReport{}, false, ctx.Err()
	}
	if s.State != nil && hash == s.State.LastHash(ctx) {
		r, _ := s.State.Last(ctx)
		return r, false, nil
	}
	r, err := s.scan(ctx, hash)
	return r, true, err
}

// ShieldScan forces a read-only scan for the explicit doctor command.
func ShieldScan(ctx context.Context, reader ports.AgentConfigReaderPort, state ports.ShieldStateRepository) domain.ShieldReport {
	if reader == nil {
		return domain.ShieldReport{Findings: []domain.ShieldFinding{}, Skipped: []string{}}
	}
	r, _ := (ShieldScanner{Reader: reader, State: state}).scan(ctx, reader.Fingerprint(ctx))
	return r
}
