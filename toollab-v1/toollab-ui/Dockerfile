FROM node:20-alpine AS builder

ARG VITE_DASHBOARD_URL=http://localhost:8090
ENV VITE_DASHBOARD_URL=$VITE_DASHBOARD_URL

WORKDIR /workspace
COPY package.json package-lock.json* ./
RUN npm install

COPY . .
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=builder /workspace/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 4173

HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -qO- http://127.0.0.1:4173/ >/dev/null 2>&1 || exit 1

CMD ["nginx", "-g", "daemon off;"]
