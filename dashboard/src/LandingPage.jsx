export default function LandingPage({ onEnter }) {
  return (
    <div className="landing">
      {/* ── Nav ── */}
      <nav className="l-nav">
        <div className="l-nav-brand">
          <img src="/logo.svg" alt="Reef Watchers" className="l-nav-logo" />
          <span>Reef Watchers</span>
        </div>
        <div className="l-nav-links">
          <a href="#features">Features</a>
          <a href="#architecture">Architecture</a>
          <a href="#specs">Tech Specs</a>
        </div>
        <button className="l-btn l-btn--sm" onClick={onEnter}>
          Launch Dashboard →
        </button>
      </nav>

      {/* ── Hero ── */}
      <section className="l-hero">
        {/* Decorative background assets */}
        <img src="/vessel.svg" alt="" className="l-hero-asset l-hero-asset--vessel" />
        <img src="/boat.svg" alt="" className="l-hero-asset l-hero-asset--boat" />

        <div className="l-hero-badge" style={{ position: 'relative', zIndex: 1 }}>
          <span className="l-badge-dot" />
          Real-Time Maritime Surveillance Engine
        </div>
        <h1 className="l-hero-title">
          Detect, Track, and Intercept
          <br />
          <span className="l-hero-accent">Dark Vessels in Real Time</span>
        </h1>
        <p className="l-hero-sub">
          Everyone alerts when a ship enters a zone. The crime happens when a
          ship <strong>disappears</strong>. We alert on absence in milliseconds,
          compute the intercept, and run on hardware a coast guard station
          already owns.
        </p>
        <div className="l-hero-actions">
          <button className="l-btn l-btn--primary" onClick={onEnter}>
            Open Live Dashboard
          </button>
          <a className="l-btn l-btn--outline" href="#features">
            Explore Features
          </a>
        </div>

        {/* Floating data cards */}
        <div className="l-hero-cards">
          <div className="l-float-card">
            <div className="l-float-label">Throughput</div>
            <div className="l-float-value accent">8.7M</div>
            <div className="l-float-unit">msgs/sec, measured</div>
          </div>
          <div className="l-float-card">
            <div className="l-float-label">Alert Latency</div>
            <div className="l-float-value warn">&lt;5ms</div>
            <div className="l-float-unit">inline p99</div>
          </div>
          <div className="l-float-card">
            <div className="l-float-label">Infrastructure</div>
            <div className="l-float-value success">Zero</div>
            <div className="l-float-unit">external deps</div>
          </div>
        </div>
      </section>

      {/* ── Stats Ticker ── */}
      <section className="l-stats">
        <div className="l-stat">
          <div className="l-stat-value accent">8.7M</div>
          <div className="l-stat-label">Msgs / Sec on Real AIS</div>
        </div>
        <div className="l-stat">
          <div className="l-stat-value warn">&lt;5ms</div>
          <div className="l-stat-label">Inline Alert Latency (p99)</div>
        </div>
        <div className="l-stat">
          <div className="l-stat-value">0</div>
          <div className="l-stat-label">Messages Dropped</div>
        </div>
        <div className="l-stat">
          <div className="l-stat-value success">1</div>
          <div className="l-stat-label">Binary, No Cloud</div>
        </div>
      </section>

      {/* ── Features: 3 Alerts ── */}
      <section id="features" className="l-features">
        <div className="l-section-header">
          <div className="l-section-tag">Core Detection</div>
          <h2 className="l-section-title">Three Alert Types. Zero Blind Spots.</h2>
          <p className="l-section-sub">
            Each alert fires at the speed of the data, not the speed of a satellite pass.
          </p>
        </div>

        <div className="l-feature-grid">
          {/* Zone Violation */}
          <div className="l-feature-card">
            <div className="l-feature-icon" style={{ borderColor: '#3987e5' }}>
              <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#3987e5" strokeWidth="2" strokeLinecap="round">
                <polygon points="12,2 22,8.5 22,15.5 12,22 2,15.5 2,8.5" />
                <line x1="12" y1="22" x2="12" y2="15.5" />
                <line x1="22" y1="8.5" x2="12" y2="15.5" />
                <line x1="2" y1="8.5" x2="12" y2="15.5" />
              </svg>
            </div>
            <h3 className="l-feature-name">Zone Violation</h3>
            <div className="l-feature-badge" style={{ color: '#3987e5', borderColor: '#3987e5' }}>
              INLINE
            </div>
            <p className="l-feature-desc">
              Fires on an outside-to-inside entry of a Marine Protected Area, or
              an EEZ crossing by a foreign-flagged vessel. A pre-rasterized grid
              skips the polygon test for the common far-from-zone message.
            </p>
            <div className="l-feature-metric">
              <span className="l-feature-metric-val">&lt;1ms</span>
              <span className="l-feature-metric-unit">per check</span>
            </div>
          </div>

          {/* Spoof Teleport */}
          <div className="l-feature-card">
            <div className="l-feature-icon" style={{ borderColor: '#c98500' }}>
              <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#c98500" strokeWidth="2" strokeLinecap="round">
                <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" />
              </svg>
            </div>
            <h3 className="l-feature-name">Spoof / Teleport</h3>
            <div className="l-feature-badge" style={{ color: '#c98500', borderColor: '#c98500' }}>
              INLINE
            </div>
            <p className="l-feature-desc">
              Flags an impossible implied speed between fixes (&gt;60 kn), or one
              MMSI reported &gt;50 nm apart within 60 seconds. Flat-plane math,
              no GIS library.
            </p>
            <div className="l-feature-metric">
              <span className="l-feature-metric-val">60kn</span>
              <span className="l-feature-metric-unit">threshold</span>
            </div>
          </div>

          {/* Dark Event */}
          <div className="l-feature-card l-feature-card--highlight">
            <div className="l-feature-icon" style={{ borderColor: '#e66767' }}>
              <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#e66767" strokeWidth="2" strokeLinecap="round">
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="8" x2="12" y2="12" />
                <line x1="12" y1="16" x2="12.01" y2="16" />
              </svg>
            </div>
            <h3 className="l-feature-name">Dark Event</h3>
            <div className="l-feature-badge" style={{ color: '#e66767', borderColor: '#e66767' }}>
              SWEEP
            </div>
            <p className="l-feature-desc">
              Catches vessels that stop transmitting: silence past 6x the expected
              interval, last speed &gt;1 kn, near a monitored zone. Emits a
              dead-reckoning cone and a patrol intercept solution. Detecting
              absence is a 1s sweep, never an inline millisecond claim.
            </p>
            <div className="l-feature-metric">
              <span className="l-feature-metric-val">1s</span>
              <span className="l-feature-metric-unit">sweep tick</span>
            </div>
          </div>
        </div>
      </section>

      {/* ── Architecture ── */}
      <section id="architecture" className="l-arch">
        <div className="l-section-header">
          <div className="l-section-tag">System Design</div>
          <h2 className="l-section-title">Single Binary. Zero Dependencies.</h2>
          <p className="l-section-sub">
            No Kafka, no Redis, no cloud. One Go process, memory is the database.
          </p>
        </div>

        <div className="l-arch-diagram">
          <div className="l-arch-row">
            <div className="l-arch-box l-arch-input">
              <div className="l-arch-box-label">Input</div>
              <div className="l-arch-box-title">AIS Feed</div>
              <div className="l-arch-box-sub">aisstream.io / synthetic</div>
            </div>
            <div className="l-arch-arrow">&rarr;</div>
            <div className="l-arch-box l-arch-engine">
              <div className="l-arch-box-label">Engine</div>
              <div className="l-arch-box-title">Ingest + Batch</div>
              <div className="l-arch-box-sub">512-msg batches, 5ms flush</div>
            </div>
            <div className="l-arch-arrow">&rarr;</div>
            <div className="l-arch-box l-arch-state">
              <div className="l-arch-box-label">State</div>
              <div className="l-arch-box-title">64 Shards</div>
              <div className="l-arch-box-sub">RWMutex, value structs</div>
            </div>
          </div>
          <div className="l-arch-down">&darr;</div>
          <div className="l-arch-row">
            <div className="l-arch-box l-arch-check">
              <div className="l-arch-box-label">Inline</div>
              <div className="l-arch-box-title">Zone + Spoof</div>
              <div className="l-arch-box-sub">per message, &lt;5ms</div>
            </div>
            <div className="l-arch-arrow">+</div>
            <div className="l-arch-box l-arch-sweep">
              <div className="l-arch-box-label">Sweep</div>
              <div className="l-arch-box-title">Dark Events</div>
              <div className="l-arch-box-sub">1s tick + intercept</div>
            </div>
            <div className="l-arch-arrow">&rarr;</div>
            <div className="l-arch-box l-arch-output">
              <div className="l-arch-box-label">Output</div>
              <div className="l-arch-box-title">WebSocket</div>
              <div className="l-arch-box-sub">alerts + metrics + GeoJSON</div>
            </div>
          </div>
        </div>
      </section>

      {/* ── Tech Specs ── */}
      <section id="specs" className="l-specs">
        <div className="l-section-header">
          <div className="l-section-tag">Under the Hood</div>
          <h2 className="l-section-title">Built for the Hot Path</h2>
        </div>

        <div className="l-specs-grid">
          <div className="l-spec-item">
            <div className="l-spec-key">Engine</div>
            <div className="l-spec-val">Go, single static binary</div>
          </div>
          <div className="l-spec-item">
            <div className="l-spec-key">Frontend</div>
            <div className="l-spec-val">React + MapLibre GL</div>
          </div>
          <div className="l-spec-item">
            <div className="l-spec-key">State Model</div>
            <div className="l-spec-val">64-shard in-memory, value structs, zero hot-path pointers</div>
          </div>
          <div className="l-spec-item">
            <div className="l-spec-key">Spatial Index</div>
            <div className="l-spec-val">0.05 degree pre-rasterized zone grid</div>
          </div>
          <div className="l-spec-item">
            <div className="l-spec-key">Dependencies</div>
            <div className="l-spec-val">gorilla/websocket, orb, zerolog</div>
          </div>
          <div className="l-spec-item">
            <div className="l-spec-key">Database</div>
            <div className="l-spec-val">None. Memory is the database.</div>
          </div>
        </div>
      </section>

      {/* ── CTA Footer ── */}
      <section className="l-cta">
        <h2 className="l-cta-title">Ready to See It Live?</h2>
        <p className="l-cta-sub">
          One binary. One laptop. Real-time dark vessel detection.
        </p>
        <button className="l-btn l-btn--primary l-btn--lg" onClick={onEnter}>
          Launch Dashboard
        </button>
      </section>

      {/* ── Footer ── */}
      <footer className="l-footer">
        <div className="l-footer-brand">
          <img src="/logo.svg" alt="Reef Watchers" className="l-footer-logo" />
          <span>Reef Watchers</span>
        </div>
        <div className="l-footer-copy">
          Built for the SOSC Hackathon. Real-time maritime dark-vessel detection engine.
        </div>
      </footer>
    </div>
  )
}
