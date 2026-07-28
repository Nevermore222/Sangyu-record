FROM node:24-alpine
WORKDIR /app
COPY skills/mock-memoir/package.json skills/mock-memoir/server.mjs ./
USER node
ENV PORT=8090
EXPOSE 8090
CMD ["node", "server.mjs"]
