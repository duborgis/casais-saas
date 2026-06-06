import express from 'express';
import cors from 'cors';
import dotenv from 'dotenv';
import { Server } from 'socket.io';
import http from 'http';
import { BedrockAgentCoreClient, InvokeAgentRuntimeCommand } from '@aws-sdk/client-bedrock-agentcore';
import { SSMClient, GetParameterCommand } from '@aws-sdk/client-ssm';
import { randomUUID } from 'crypto';

dotenv.config();

const app = express();
const server = http.createServer(app);
const io = new Server(server, {
  cors: { origin: '*', methods: ['GET', 'POST'] },
});

app.use(cors());
app.use(express.json());

const AWS_REGION = process.env.AWS_REGION || 'us-east-1';
const ORCHESTRATOR_ARN = process.env.ORCHESTRATOR_ARN || '';
const CREW_CONFIG_SSM_PATH = process.env.CREW_CONFIG_SSM_PATH || '/casais-saas/agents/crew-config';

const agentClient = new BedrockAgentCoreClient({ region: AWS_REGION });
const ssmClient = new SSMClient({ region: AWS_REGION });

// Cache the crew config — changes only on deploy
let _crewConfigCache = null;
async function getCrewConfig() {
  if (_crewConfigCache) return _crewConfigCache;
  try {
    const res = await ssmClient.send(new GetParameterCommand({ Name: CREW_CONFIG_SSM_PATH }));
    _crewConfigCache = JSON.parse(res.Parameter.Value);
    console.log('[i] Crew config loaded from SSM:', _crewConfigCache.name);
  } catch (err) {
    console.error('[!] Could not load crew config from SSM:', err.message);
    _crewConfigCache = null;
  }
  return _crewConfigCache;
}

app.get('/health', (_req, res) => {
  res.json({ status: 'ok', service: 'casais-saas-backend' });
});

io.on('connection', (socket) => {
  console.log('[*] Novo usuário conectado no Harmonia');

  socket.on('message', async (data) => {
    const { text, userId = 'anonymous', sessionId = randomUUID(), coupleId = '' } = data;

    if (!ORCHESTRATOR_ARN) {
      socket.emit('error', { message: 'ORCHESTRATOR_ARN não configurado.' });
      return;
    }

    try {
      const crewConfig = await getCrewConfig();
      const runId = randomUUID();

      const payload = {
        sessionId,
        input: {
          text,
          userId,
          agentId: 'orchestrator',
          runtimeContext: {
            runId,
            conversationId: coupleId || sessionId,
            delegationDepth: 0,
          },
          ...(crewConfig ? {
            agentConfig: { id: 'orchestrator', runtime_arn: ORCHESTRATOR_ARN },
            crewConfig,
          } : {}),
        },
      };

      console.log(`[>] Invocando orquestrador | session=${sessionId} user=${userId}`);

      const cmd = new InvokeAgentRuntimeCommand({
        agentRuntimeArn: ORCHESTRATOR_ARN,
        qualifier: 'DEFAULT',
        contentType: 'application/json',
        accept: 'application/json',
        payload: Buffer.from(JSON.stringify(payload)),
      });

      const response = await agentClient.send(cmd);
      const body = await response.response.transformToString();
      const result = JSON.parse(body);
      const responseText = result?.output?.text || '';

      socket.emit('agent_complete', { fullText: responseText, sessionId });
      console.log(`[<] Resposta recebida | session=${sessionId} chars=${responseText.length}`);

    } catch (err) {
      console.error('[!] Erro ao invocar orquestrador:', err.message);
      socket.emit('error', { message: 'Falha na comunicação com o agente. Tente novamente.' });
    }
  });
});

const PORT = process.env.PORT || 3001;
server.listen(PORT, () => {
  console.log(`[OK] Harmonia backend rodando na porta ${PORT}`);
  console.log(`[i] Orquestrador ARN: ${ORCHESTRATOR_ARN || '(não configurado)'}`);
});
