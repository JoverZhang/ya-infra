FROM docker.io/library/node@sha256:54f84b4cf8432b21aaffd5d3ce8d41e12cea0d10d33c09d2f3c9879d49d6b446
WORKDIR /opt/annotations
RUN npm install --omit=dev agentation-mcp@1.3.2 \
    && mkdir -p /home/node/.agentation \
    && chown node:node /home/node/.agentation
USER node
CMD ["node", "--input-type=module", "-e", "import { startHttpServer } from 'agentation-mcp'; startHttpServer(4747);"]
