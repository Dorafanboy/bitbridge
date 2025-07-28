import React from 'react';
import './App.css';

function App() {
  return (
    <div className="App">
      <header className="App-header">
        <h1>UTXO-EVM Gateway</h1>
        <p>Bitcoin to Ethereum Bridge</p>
        <div className="feature-grid">
          <div className="feature-card">
            <h3>>™ Bitcoin ’ Ethereum</h3>
            <p>Convert Bitcoin UTXOs to ERC-20 tokens</p>
          </div>
          <div className="feature-card">
            <h3>=± DEX Trading</h3>
            <p>Trade UTXO tokens via 1inch protocol</p>
          </div>
          <div className="feature-card">
            <h3>= Token ’ Bitcoin</h3>
            <p>Burn tokens to redeem original Bitcoin</p>
          </div>
        </div>
        <div className="status-section">
          <h3>Gateway Status</h3>
          <div className="status-indicator">
            <span className="status-dot"></span>
            <span>Backend: Connecting...</span>
          </div>
          <div className="status-indicator">
            <span className="status-dot"></span>
            <span>Bitcoin Network: Testnet</span>
          </div>
          <div className="status-indicator">
            <span className="status-dot"></span>
            <span>Ethereum Network: Sepolia</span>
          </div>
        </div>
      </header>
    </div>
  );
}

export default App;