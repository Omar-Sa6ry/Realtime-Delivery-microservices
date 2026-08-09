FROM node:24.11.0

WORKDIR /usr/src/app

COPY packages/ts ./packages/ts
COPY services/user-service/package.json ./services/user-service/

WORKDIR /usr/src/app/services/user-service
RUN npm install --legacy-peer-deps

WORKDIR /usr/src/app
COPY . .

WORKDIR /usr/src/app/services/user-service
CMD ["npm", "run", "start:dev"]
