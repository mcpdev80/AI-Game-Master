FROM node:22.22.1-bookworm-slim AS build

WORKDIR /app

ARG APP_VERSION=0.1.0
ARG GIT_COMMIT=dev
ARG BUILD_DATE=unknown
ENV NEXT_PUBLIC_APP_VERSION=$APP_VERSION
ENV NEXT_PUBLIC_GIT_COMMIT=$GIT_COMMIT
ENV NEXT_PUBLIC_BUILD_DATE=$BUILD_DATE

COPY package.json ./
RUN npm install

COPY . .
RUN npm run build

FROM node:22.22.1-bookworm-slim

WORKDIR /app
ENV NODE_ENV=production

COPY --from=build /app/public ./public
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static

EXPOSE 3000

CMD ["node", "server.js"]
