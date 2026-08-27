# The React bundle, served by nginx with the API proxied from the same origin.

FROM node:24-alpine AS build
WORKDIR /app

# npm ci needs both files and installs exactly the lockfile, so a container
# build cannot quietly pick up a newer transitive version than the one that
# was tested.
COPY package.json package-lock.json ./
RUN npm ci

COPY . .

# Vite inlines this at build time — it is not readable at runtime, which is
# the whole reason nginx proxies the API rather than the bundle calling an
# absolute URL. The relative default works from every address the instance
# can be reached at; override it only to point the bundle at an API on a
# different origin, and then CORS_ALLOWED_ORIGINS has to allow this one.
ARG VITE_API_BASE_URL=/api/v1
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL

RUN npm run build

FROM nginx:1.27-alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
