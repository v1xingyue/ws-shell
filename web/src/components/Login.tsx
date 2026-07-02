import { useState, useEffect } from 'react';

interface LoginProps {
  onLogin: (user: { id: string; username: string }) => void;
}

const Login = ({ onLogin }: LoginProps) => {
  const [isLoading, setIsLoading] = useState(true);
  const [authMethods, setAuthMethods] = useState<string[]>([]);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    // 检查是否已经登录
    checkAuthStatus();
  }, []);

  const checkAuthStatus = async () => {
    try {
      const response = await fetch('/auth/me');
      if (response.ok) {
        const user = await response.json();
        onLogin(user);
      } else {
        const data = await response.json().catch(() => ({}));
        setAuthMethods(data.auth_methods || []);
      }
    } catch (err) {
      console.log('Not authenticated');
    } finally {
      setIsLoading(false);
    }
  };

  const handleGitHubLogin = () => {
    window.location.href = '/auth/github';
  };

  const handlePasswordLogin = async (event: React.FormEvent) => {
    event.preventDefault();
    setError('');
    const response = await fetch('/auth/password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    if (response.ok) {
      onLogin(await response.json());
      return;
    }
    setError('Invalid username or password');
  };

  if (isLoading) {
    return (
      <div style={styles.container}>
        <div style={styles.loading}>Checking authentication...</div>
      </div>
    );
  }

  return (
    <div style={styles.container}>
      <div style={styles.loginBox}>
        <h1 style={styles.title}>Terminal Access</h1>
        <p style={styles.subtitle}>Please sign in to continue</p>

        {authMethods.includes('password') && (
          <form style={styles.form} onSubmit={handlePasswordLogin}>
            <input
              style={styles.input}
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              placeholder="Username"
              autoComplete="username"
            />
            <input
              style={styles.input}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder="Password"
              type="password"
              autoComplete="current-password"
            />
            {error && <div style={styles.error}>{error}</div>}
            <button style={styles.passwordButton} type="submit">Sign in</button>
          </form>
        )}

        {authMethods.includes('github') && (
          <button style={styles.githubButton} onClick={handleGitHubLogin}>
            <svg style={styles.githubIcon} viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
            </svg>
            Sign in with GitHub
          </button>
        )}

        <p style={styles.info}>
          Only authorized users can access this terminal.
        </p>
      </div>
    </div>
  );
};

const styles: { [key: string]: React.CSSProperties } = {
  container: {
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    minHeight: '100vh',
    background: 'linear-gradient(135deg, #1e1e1e 0%, #2d2d2d 100%)',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
  },
  loginBox: {
    background: '#252526',
    padding: '40px 60px',
    borderRadius: '12px',
    boxShadow: '0 8px 32px rgba(0, 0, 0, 0.3)',
    textAlign: 'center',
    maxWidth: '400px',
    width: '100%',
  },
  title: {
    color: '#ffffff',
    fontSize: '28px',
    marginBottom: '8px',
    fontWeight: 600,
  },
  subtitle: {
    color: '#cccccc',
    fontSize: '16px',
    marginBottom: '32px',
  },
  form: {
    display: 'flex',
    flexDirection: 'column',
    gap: '12px',
    marginBottom: '16px',
  },
  input: {
    width: '100%',
    boxSizing: 'border-box',
    padding: '12px 14px',
    backgroundColor: '#1e1e1e',
    color: '#ffffff',
    border: '1px solid #3c3c3c',
    borderRadius: '6px',
    fontSize: '16px',
  },
  passwordButton: {
    width: '100%',
    padding: '14px 24px',
    backgroundColor: '#0e639c',
    color: '#ffffff',
    border: 'none',
    borderRadius: '6px',
    fontSize: '16px',
    fontWeight: 600,
    cursor: 'pointer',
  },
  githubButton: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: '12px',
    width: '100%',
    padding: '14px 24px',
    backgroundColor: '#ffffff',
    color: '#24292e',
    border: 'none',
    borderRadius: '8px',
    fontSize: '16px',
    fontWeight: 600,
    cursor: 'pointer',
    transition: 'background-color 0.2s',
    marginTop: '8px',
  },
  githubIcon: {
    width: '20px',
    height: '20px',
  },
  info: {
    color: '#888888',
    fontSize: '14px',
    marginTop: '24px',
  },
  error: {
    color: '#f48771',
    fontSize: '14px',
    textAlign: 'left',
  },
  loading: {
    color: '#cccccc',
    fontSize: '16px',
  },
};

export default Login;
