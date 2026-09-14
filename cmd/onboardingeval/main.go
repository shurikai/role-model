// Command onboardingeval drives the #117 onboarding agent end to end against
// a real narrative and a real LLM playing the candidate, then reports what
// actually landed in the database.
//
// It exists because #117's own acceptance criteria include an eval: "run it
// against the existing seeded career data, where the right answers are
// known, and judge whether it asks good questions and writes accurate
// contributions." Nothing exercised that path before this.
//
// Modelled closely on cmd/intakerun: a throwaway account, a real model call,
// explicit invocation only. It is deliberately not wired into the server or
// CI -- it spends money and drives a live interview, and both of those want
// an explicit run.
//
// The account is created through POST /auth/signup and the interview driven
// entirely through POST .../onboarding/turns -- the same two routes the
// frontend calls, so this tool is a client of the running server rather than
// a shortcut around it, and needs no copy of JWT_SECRET to mint its own
// token (which turned out to matter: a long-running dev server can hold an
// in-memory secret that no longer matches whatever is currently in .env --
// godotenv.Load() never overrides an already-set environment variable --
// and minting a token against the wrong copy failed with a plain 401 that
// gave no hint why). Reading the result back for the report, and deleting
// the account afterward, are the only places this connects to Postgres
// directly, the same way cmd/clearhistory and cmd/intakerun's own account
// setup do; that is about how THIS tool inspects and tears down its own
// outcome, not about how the onboarding agent writes.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shurikai/role-model/internal/db"
	"github.com/shurikai/role-model/internal/generation"
	"github.com/shurikai/role-model/internal/onboarding"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "onboardingeval: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		file        = flag.String("file", "database/sample-clinical/career-notes.txt", "path to the career narrative to feed the simulated candidate")
		email       = flag.String("email", "", "email for the throwaway account (default: timestamp-suffixed)")
		apiURL      = flag.String("api-url", "http://localhost:8080/api/v1", "base URL of the running Go API")
		maxTurns    = flag.Int("max-turns", 60, "safety cap on interview turns -- a loud failure if the loop runs away")
		keepAccount = flag.Bool("keep-account", false, "leave the throwaway account and its data in place instead of deleting it")
	)
	flag.Parse()

	if *email == "" {
		*email = fmt.Sprintf("onboardingeval-%d@example.local", time.Now().Unix())
	}

	dsn := os.Getenv("DATABASE_URL")
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if dsn == "" || apiKey == "" {
		return fmt.Errorf("DATABASE_URL and ANTHROPIC_API_KEY are both required")
	}

	notes, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("read %s: %w", *file, err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()
	q := db.New(pool)

	client := &onboardingClient{baseURL: strings.TrimSuffix(*apiURL, "/"), http: &http.Client{Timeout: 60 * time.Second}}

	password, err := randomPassword()
	if err != nil {
		return fmt.Errorf("generate password: %w", err)
	}
	// Through POST /auth/signup, not a direct Postgres write: the account
	// this creates -- and the token it comes back with -- are exactly what a
	// real second user gets, and the server mints its own token rather than
	// this tool needing a correct, currently-live copy of JWT_SECRET to mint
	// one that matches. Requires SIGNUP_ENABLED=true on the target server,
	// which is development's default.
	userID, err := client.signup(ctx, *email, password)
	if err != nil {
		return fmt.Errorf("signup (is SIGNUP_ENABLED=true on the target server?): %w", err)
	}
	log.Printf("account %s (%s) created", *email, userID)

	if !*keepAccount {
		defer func() {
			if err := deleteAccount(context.Background(), pool, userID); err != nil {
				log.Printf("cleanup: %v", err)
			} else {
				log.Printf("account %s deleted", userID)
			}
		}()
	}
	candidate := newSimulatedCandidate(generation.NewClient(apiKey), string(notes))

	// A failed turn does not skip the report: whatever was written to the
	// database before the failure, and the transcript up to that point, are
	// themselves a real finding -- a crash partway through is exactly the
	// kind of thing this tool exists to surface, not something to let a
	// stack trace hide. The interview error (if any) is reported after,
	// and still fails the run's exit code.
	sessionID, interviewErr := driveInterview(ctx, client, candidate, *maxTurns)
	if interviewErr != nil {
		log.Printf("interview did not finish: %v", interviewErr)
	} else {
		log.Printf("interview %s finished after %d turns", sessionID, len(candidate.transcript))
	}

	if err := printReport(ctx, q, userID, candidate.transcript); err != nil {
		return fmt.Errorf("report: %w", err)
	}
	if interviewErr != nil {
		return fmt.Errorf("interview: %w", interviewErr)
	}
	return nil
}

// driveInterview runs turns until the agent reports done, capped at
// maxTurns. Returns the session id for logging.
func driveInterview(ctx context.Context, client *onboardingClient, candidate *simulatedCandidate, maxTurns int) (string, error) {
	var sessionID string
	var message *string

	for turn := 0; ; turn++ {
		if turn >= maxTurns {
			return sessionID, fmt.Errorf("exceeded -max-turns (%d) without the interview finishing", maxTurns)
		}

		var sessionIDArg *string
		if sessionID != "" {
			sessionIDArg = &sessionID
		}
		resp, err := client.turn(ctx, sessionIDArg, message)
		if err != nil {
			return sessionID, fmt.Errorf("turn %d: %w", turn+1, err)
		}
		sessionID = resp.SessionID

		if resp.Done {
			return sessionID, nil
		}

		answer, err := candidate.answer(ctx, resp.Reply)
		if err != nil {
			return sessionID, fmt.Errorf("simulated candidate, turn %d: %w", turn+1, err)
		}
		message = &answer
	}
}

// qaTurn is one question-answer pair, kept for both the candidate's own
// growing context and the final printed transcript.
type qaTurn struct {
	Question string
	Answer   string
}

// simulatedCandidate plays the person behind the career notes, answering the
// onboarding agent's questions one at a time using generation.Client.Complete
// -- the same single system+user-message helper intake.Extractor is stubbed
// against elsewhere, reused here instead of hand-rolling new Anthropic SDK
// plumbing for a multi-turn conversation. Multi-turn is simulated by folding
// the transcript so far into the user content on every call, since Complete
// itself only ever sends one user message.
type simulatedCandidate struct {
	client       *generation.Client
	systemPrompt string
	transcript   []qaTurn
}

func newSimulatedCandidate(client *generation.Client, careerNotes string) *simulatedCandidate {
	systemPrompt := fmt.Sprintf(`You are roleplaying as the person who wrote the career notes below, answering an onboarding interviewer's questions about your own work history one at a time.

Rules:
- Answer ONLY using what is stated in the notes. Never invent a fact, date, employer, or accomplishment that isn't there.
- If the interviewer asks about something the notes don't cover, say so plainly instead of making something up.
- Answer naturally, the way a real person talks in an interview -- a sentence or two, not a list, not overly formal.
- Answer only the question just asked. Do not volunteer your whole career history at once.
- Once you've covered everything in the notes and the interviewer asks if there's anything else, say so -- don't repeat yourself or pad an answer to seem more complete than the notes support.

Your career notes:

%s`, careerNotes)

	return &simulatedCandidate{client: client, systemPrompt: systemPrompt}
}

// answer asks the candidate to respond to the interviewer's latest question,
// given everything asked and answered so far, and records the exchange.
func (c *simulatedCandidate) answer(ctx context.Context, question string) (string, error) {
	var b strings.Builder
	for i, t := range c.transcript {
		fmt.Fprintf(&b, "Interviewer (turn %d): %s\nYou answered: %s\n\n", i+1, t.Question, t.Answer)
	}
	fmt.Fprintf(&b, "Interviewer's new question: %s\n\nYour answer:", question)

	answer, err := c.client.Complete(ctx, c.systemPrompt, b.String(), 512)
	if err != nil {
		return "", err
	}
	c.transcript = append(c.transcript, qaTurn{Question: question, Answer: answer})
	// Printed live, not just in the final report -- so a run that crashes
	// mid-interview still leaves the conversation on screen, and so it's
	// watchable while running rather than only readable after the fact.
	log.Printf("Q%d: %s\n    A: %s", len(c.transcript), question, answer)
	return answer, nil
}

// onboardingClient calls the PUBLIC onboarding route -- the same one the
// frontend calls, not internal/onboarding.Client (which is the Go server's
// OWN internal client for reaching the Python service directly, a different
// base path and a different auth shape entirely).
type onboardingClient struct {
	baseURL string
	token   string
	http    *http.Client
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string `json:"token"`
	User  struct {
		ID uuid.UUID `json:"id"`
	} `json:"user"`
}

// signup creates the throwaway account through the real signup route and
// stores the token it returns on the client for subsequent calls. Returns
// the new account's id.
func (c *onboardingClient) signup(ctx context.Context, email, password string) (uuid.UUID, error) {
	body, err := json.Marshal(authRequest{Email: email, Password: password})
	if err != nil {
		return uuid.Nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/signup", bytes.NewReader(body))
	if err != nil {
		return uuid.Nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return uuid.Nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return uuid.Nil, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return uuid.Nil, fmt.Errorf("status %d: %s", resp.StatusCode, respBody)
	}

	var auth authResponse
	if err := json.Unmarshal(respBody, &auth); err != nil {
		return uuid.Nil, fmt.Errorf("parse response: %w", err)
	}
	c.token = auth.Token
	return auth.User.ID, nil
}

// randomPassword is only ever used once, immediately, by this same process --
// there is no need to log in again -- so a locally-generated throwaway value
// is simpler than adding another required env var for something nobody
// needs to remember.
func randomPassword() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

type turnRequest struct {
	SessionID *string `json:"session_id,omitempty"`
	Message   *string `json:"message,omitempty"`
}

func (c *onboardingClient) turn(ctx context.Context, sessionID, message *string) (onboarding.TurnResponse, error) {
	body, err := json.Marshal(turnRequest{SessionID: sessionID, Message: message})
	if err != nil {
		return onboarding.TurnResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/onboarding/turns", bytes.NewReader(body))
	if err != nil {
		return onboarding.TurnResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return onboarding.TurnResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return onboarding.TurnResponse{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return onboarding.TurnResponse{}, fmt.Errorf("status %d: %s", resp.StatusCode, respBody)
	}

	var turn onboarding.TurnResponse
	if err := json.Unmarshal(respBody, &turn); err != nil {
		return onboarding.TurnResponse{}, fmt.Errorf("parse response: %w", err)
	}
	return turn, nil
}

// deleteAccount removes everything a run of this tool could have created,
// in FK-safe order, plus the account itself and its starting vocabulary.
// Deliberately narrower than cmd/clearhistory's table list: this account
// only ever goes through the onboarding agent, which writes to none of
// clearhistory's other tables (applications, education, preferences,
// projects, ...), so reusing that broader list here would be reaching for
// tables this tool never touches. One transaction, all or nothing.
func deleteAccount(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID) error {
	stmts := []string{
		`DELETE FROM contribution_tags WHERE contribution_id IN (SELECT id FROM contributions WHERE user_id = $1)`,
		`DELETE FROM skills WHERE user_id = $1`,
		`DELETE FROM contributions WHERE user_id = $1`,
		`DELETE FROM positions WHERE user_id = $1`,
		`DELETE FROM employers WHERE user_id = $1`,
		`DELETE FROM tags WHERE user_id = $1`,
		`DELETE FROM tag_categories WHERE user_id = $1`,
		`DELETE FROM career_levels WHERE user_id = $1`,
		`DELETE FROM proficiency_levels WHERE user_id = $1`,
		`DELETE FROM resume_sections WHERE user_id = $1`,
		`DELETE FROM users WHERE id = $1`,
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // no-op after a successful commit

	for _, stmt := range stmts {
		if _, err := tx.Exec(ctx, stmt, userID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
