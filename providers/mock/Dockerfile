FROM node:24-alpine
WORKDIR /app
COPY providers/mock/package.json providers/mock/server.mjs ./
USER node
EXPOSE 8090
CMD ["node", "server.mjs"]
