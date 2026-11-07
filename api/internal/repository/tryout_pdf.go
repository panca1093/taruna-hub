package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/tarunaeduka/bimbel-tryout/api/internal/model"
)

// PrintTryout reads a tryout in the order the admin stored it, through the frozen version of each assignment.
// The query names stem and option labels only; keys, explanations and option points are never selected.
func (r *TryoutRepository) PrintTryout(ctx context.Context, tenantID, id uuid.UUID) (model.PrintTryout, error) {
	var out model.PrintTryout
	err := r.db.QueryRow(ctx,
		`SELECT t.title, t.revision, n.name FROM tryouts t JOIN tenants n ON n.id = t.tenant_id
		  WHERE t.tenant_id = $1 AND t.id = $2`, tenantID, id).Scan(&out.Title, &out.Revision, &out.TenantName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.PrintTryout{}, ErrNotFound
		}
		return model.PrintTryout{}, fmt.Errorf("read print tryout: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT s.id, s.name, s.duration_seconds, v.content,
		       COALESCE((SELECT jsonb_agg(o.obj ->> 'label' ORDER BY o.ord)
		                   FROM jsonb_array_elements(v.options) WITH ORDINALITY AS o(obj, ord)), '[]'::jsonb)
		  FROM tryout_subtests s
		  LEFT JOIN tryout_questions tq ON tq.subtest_id = s.id AND tq.tenant_id = s.tenant_id
		  LEFT JOIN question_versions v ON v.id = tq.question_version_id AND v.tenant_id = tq.tenant_id
		 WHERE s.tenant_id = $1 AND s.tryout_id = $2
		 ORDER BY s.position, tq.position`, tenantID, id)
	if err != nil {
		return model.PrintTryout{}, fmt.Errorf("read print questions: %w", err)
	}
	defer rows.Close()
	index := map[uuid.UUID]int{}
	for rows.Next() {
		var subtestID uuid.UUID
		var name string
		var duration int
		var content *string
		var rawOptions []byte
		if err := rows.Scan(&subtestID, &name, &duration, &content, &rawOptions); err != nil {
			return model.PrintTryout{}, fmt.Errorf("scan print question: %w", err)
		}
		at, seen := index[subtestID]
		if !seen {
			at = len(out.Subtests)
			index[subtestID] = at
			out.Subtests = append(out.Subtests, model.PrintSubtest{Name: name, DurationSeconds: duration})
		}
		if content == nil {
			continue
		}
		var labels []string
		if err := json.Unmarshal(rawOptions, &labels); err != nil {
			return model.PrintTryout{}, fmt.Errorf("decode print options: %w", err)
		}
		out.Subtests[at].Questions = append(out.Subtests[at].Questions, model.PrintQuestion{Content: *content, Options: labels})
	}
	if err := rows.Err(); err != nil {
		return model.PrintTryout{}, fmt.Errorf("read print questions: %w", err)
	}
	return out, nil
}
