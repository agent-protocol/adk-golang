// Package api provides web UI functionality for the ADK web server
package api

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed static/*
var staticFiles embed.FS

// WebUIHandler provides web UI functionality
type WebUIHandler struct {
	server *Server
}

// NewWebUIHandler creates a new web UI handler
func NewWebUIHandler(server *Server) *WebUIHandler {
	return &WebUIHandler{server: server}
}

// ServeStaticFiles serves embedded static files
func (w *WebUIHandler) ServeStaticFiles(pattern string, mux *http.ServeMux) {
	fileServer := http.FileServer(http.FS(staticFiles))
	mux.Handle(pattern, http.StripPrefix("/static/", fileServer))
}

// HandleIndex serves the main web UI page
func (w *WebUIHandler) HandleIndex(writer http.ResponseWriter, req *http.Request) {
	// Simple HTML template for the web UI
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ADK Agent Development Kit</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px;
            text-align: center;
        }
        .header h1 {
            margin: 0;
            font-size: 2.5em;
        }
        .header p {
            margin: 10px 0 0 0;
            opacity: 0.9;
        }
        .main-content {
            display: flex;
            min-height: 600px;
        }
        .sidebar {
            width: 300px;
            background: #f8f9fa;
            border-right: 1px solid #e9ecef;
            padding: 20px;
        }
        .chat-area {
            flex: 1;
            display: flex;
            flex-direction: column;
            padding: 20px;
        }
        .agent-list {
            margin-bottom: 20px;
        }
        .agent-list h3 {
            margin: 0 0 10px 0;
            color: #333;
        }
        .agent-item {
            padding: 10px;
            margin: 5px 0;
            background: white;
            border: 1px solid #ddd;
            border-radius: 4px;
            cursor: pointer;
            transition: background-color 0.2s;
        }
        .agent-item:hover {
            background: #e9ecef;
        }
        .agent-item.active {
            background: #007bff;
            color: white;
        }
        .chat-messages {
            flex: 1;
            border: 1px solid #ddd;
            border-radius: 4px;
            padding: 20px;
            margin-bottom: 20px;
            overflow-y: auto;
            background: #fafafa;
        }
        .message {
            margin: 10px 0;
            padding: 10px;
            border-radius: 8px;
        }
        .message.user {
            background: #007bff;
            color: white;
            margin-left: 20%;
        }
        .message.agent {
            background: white;
            border: 1px solid #ddd;
            margin-right: 20%;
        }
        .input-area {
            display: flex;
            gap: 10px;
        }
        .input-area input {
            flex: 1;
            padding: 12px;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 16px;
        }
        .input-area button {
            padding: 12px 24px;
            background: #007bff;
            color: white;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 16px;
        }
        .input-area button:hover {
            background: #0056b3;
        }
        .input-area button:disabled {
            background: #6c757d;
            cursor: not-allowed;
        }
        .status {
            padding: 10px;
            margin: 10px 0;
            border-radius: 4px;
            background: #d4edda;
            border: 1px solid #c3e6cb;
            color: #155724;
        }
        .error {
            background: #f8d7da;
            border: 1px solid #f5c6cb;
            color: #721c24;
        }
        .loading {
            display: none;
            text-align: center;
            color: #6c757d;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🤖 ADK Agent Chat</h1>
            <p>Chat with your AI agents locally</p>
        </div>
        <div class="main-content">
            <div class="sidebar">
                <div class="agent-list">
                    <h3>Available Agents</h3>
                    <div id="agents-container">
                        <div class="loading">Loading agents...</div>
                    </div>
                </div>
                <div class="status" id="status">
                    Ready to chat! Select an agent to get started.
                </div>
            </div>
            <div class="chat-area">
                <div class="chat-messages" id="messages"></div>
                <div class="input-area">
                    <input type="text" id="messageInput" placeholder="Type your message here..." disabled>
                    <button onclick="sendMessage()" id="sendButton" disabled>Send</button>
                </div>
            </div>
        </div>
    </div>

    <script>
        let currentAgent = null;
        let currentSession = null;
        let isLoading = false;

        // Load available agents
        async function loadAgents() {
            try {
                const response = await fetch('/list-apps');
                const agents = await response.json();
                
                const container = document.getElementById('agents-container');
                container.innerHTML = '';
                
                if (agents.length === 0) {
                    container.innerHTML = '<div class="agent-item">No agents found</div>';
                    return;
                }
                
                agents.forEach(agent => {
                    const item = document.createElement('div');
                    item.className = 'agent-item';
                    item.textContent = agent;
                    item.onclick = () => selectAgent(agent);
                    container.appendChild(item);
                });
            } catch (error) {
                showError('Failed to load agents: ' + error.message);
            }
        }

        // Select an agent
        async function selectAgent(agentName) {
            // Update UI
            document.querySelectorAll('.agent-item').forEach(item => {
                item.classList.remove('active');
            });
            event.target.classList.add('active');
            
            currentAgent = agentName;
            
            // Create a new session
            try {
                const response = await fetch(` + "`" + `/apps/${agentName}/users/test-user/sessions` + "`" + `, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({})
                });
                
                const session = await response.json();
                currentSession = session.id;
                
                // Clear messages and enable input
                document.getElementById('messages').innerHTML = '';
                document.getElementById('messageInput').disabled = false;
                document.getElementById('sendButton').disabled = false;
                document.getElementById('messageInput').focus();
                
                showStatus(` + "`" + `Connected to agent: ${agentName}` + "`" + `);
            } catch (error) {
                showError('Failed to create session: ' + error.message);
            }
        }

        // Send a message
        async function sendMessage() {
            const input = document.getElementById('messageInput');
            const message = input.value.trim();
            
            if (!message || !currentAgent || !currentSession || isLoading) {
                return;
            }
            
            input.value = '';
            input.disabled = true;
            document.getElementById('sendButton').disabled = true;
            isLoading = true;
            
            // Add user message to chat
            addMessage('user', message);
            showStatus('Agent is thinking...');
            
            try {
                const response = await fetch('/run_sse', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({
                        app_name: currentAgent,
                        user_id: 'test-user',
                        session_id: currentSession,
                        new_message: {
                            role: 'user',
                            parts: [{ type: 'text', text: message }]
                        },
                        streaming: true
                    })
                });

                if (!response.ok) {
                    throw new Error(` + "`" + `HTTP ${response.status}: ${response.statusText}` + "`" + `);
                }

                const reader = response.body.getReader();
                const decoder = new TextDecoder();
                let agentMessageDiv = null;

                while (true) {
                    const { done, value } = await reader.read();
                    if (done) break;

                    const chunk = decoder.decode(value);
                    const lines = chunk.split('\n');

                    for (const line of lines) {
                        if (line.startsWith('data: ')) {
                            try {
                                const data = JSON.parse(line.slice(6));
                                if (data.error) {
                                    throw new Error(data.error);
                                }
                                
                                // Handle agent response
                                if (data.author && data.author !== 'user' && data.content) {
                                    const text = extractTextFromContent(data.content);
                                    if (text) {
                                        if (!agentMessageDiv) {
                                            agentMessageDiv = addMessage('agent', '');
                                        }
                                        agentMessageDiv.textContent += text;
                                    }
                                }
                            } catch (e) {
                                console.error('Failed to parse SSE data:', e);
                            }
                        }
                    }
                }
            } catch (error) {
                addMessage('agent', 'Error: ' + error.message);
                showError('Failed to send message: ' + error.message);
            } finally {
                input.disabled = false;
                document.getElementById('sendButton').disabled = false;
                input.focus();
                isLoading = false;
                showStatus(` + "`" + `Connected to agent: ${currentAgent}` + "`" + `);
            }
        }

        // Extract text from content object
        function extractTextFromContent(content) {
            if (content.parts) {
                for (const part of content.parts) {
                    if (part.text) {
                        return part.text;
                    }
                }
            }
            return '';
        }

        // Add message to chat
        function addMessage(type, text) {
            const messages = document.getElementById('messages');
            const messageDiv = document.createElement('div');
            messageDiv.className = ` + "`" + `message ${type}` + "`" + `;
            messageDiv.textContent = text;
            messages.appendChild(messageDiv);
            messages.scrollTop = messages.scrollHeight;
            return messageDiv;
        }

        // Show status message
        function showStatus(message) {
            const status = document.getElementById('status');
            status.textContent = message;
            status.className = 'status';
        }

        // Show error message
        function showError(message) {
            const status = document.getElementById('status');
            status.textContent = message;
            status.className = 'status error';
        }

        // Handle Enter key in input
        document.getElementById('messageInput').addEventListener('keypress', function(e) {
            if (e.key === 'Enter' && !isLoading) {
                sendMessage();
            }
        });

        // Load agents on page load
        loadAgents();
    </script>
</body>
</html>`

	t, err := template.New("index").Parse(tmpl)
	if err != nil {
		http.Error(writer, "Template error", http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", "text/html")
	t.Execute(writer, nil)
}

// SetupWebRoutes adds web UI routes to the server
func (s *Server) SetupWebRoutes() {
	webUI := NewWebUIHandler(s)

	// Serve the main web UI page
	s.router.HandleFunc("/", webUI.HandleIndex)

	// Serve static files (CSS, JS, images)
	webUI.ServeStaticFiles("/static/", s.router)
}
