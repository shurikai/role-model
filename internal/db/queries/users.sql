-- name: GetUser :one
SELECT id, email, full_name, phone, location,
       linkedin_url, github_url, site_url, headline,
       created_at, updated_at
FROM users
WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (id, email, password_hash)
VALUES ($1, $2, $3)
RETURNING id, email, full_name, phone, location,
          linkedin_url, github_url, site_url, headline,
          created_at, updated_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash
FROM users
WHERE email = $1;
