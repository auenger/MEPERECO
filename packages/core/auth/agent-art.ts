// The same vector artwork is rendered by the browser and react-native-svg.
// Each character has a separate rear panel so the privacy pose reads at small sizes.
function illustration(viewBox: string, drawing: string) {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${viewBox}" stroke-linecap="round" stroke-linejoin="round">${drawing}</svg>`;
}

function terminal(away: boolean) {
  return illustration("0 0 126 132", `
    <g stroke="#192d50" stroke-width="3">
      <path d="M53 29l-6-17" fill="none"/><circle cx="45" cy="9" r="5" fill="#ffc76a"/>
      <path d="M32 87q-15 1-19 14m78-13q12 1 18-7" fill="none" stroke-width="7"/>
      <circle cx="13" cy="102" r="5" fill="#83b1ff"/><circle cx="110" cy="80" r="5" fill="#83b1ff"/>
      <rect x="21" y="25" width="77" height="75" rx="14" fill="#2860c7"/>
      <rect x="28" y="22" width="74" height="74" rx="13" fill="#649bff"/>
      <path d="M38 27h47" stroke="#a7ceff"/>
      ${away ? `<rect x="39" y="35" width="51" height="43" rx="9" fill="#5489e4"/>
        <path d="M50 48h28m-28 8h28m-28 8h28" stroke="#2a559b"/>` : `
        <rect x="36" y="36" width="58" height="41" rx="11" fill="#192e51"/>
        <path d="M43 48q0-6 7-6h15" fill="none" stroke="#3a5274" stroke-width="2"/>
        <path d="M56 52v7m21-7v7" stroke="#b6ffe8" stroke-width="6"/>
        <path d="M63 65q4 3 8-1" fill="none" stroke="#b6ffe8" stroke-width="2.5"/>
        <circle cx="88" cy="85" r="2" stroke="none" fill="#cfebff"/><path d="M40 85h13" stroke="#325ea9"/>`}
      <path d="M43 97l-6 6 4 10m38-16 6 6-4 10" stroke-width="8"/>
      <path d="M40 113q-11 0-13 7 5 4 19 2l1-7zm41-1q10 0 13 6-2 5-17 5l-1-8z" fill="#233958"/>
    </g>`);
}

function courier(away: boolean) {
  return illustration("0 0 126 132", `
    <g stroke="#453326" stroke-width="3">
      <path d="M79 45q6-12 17-15" fill="none"/><circle cx="98" cy="29" r="5" fill="#ffe9ab"/>
      <path d="M30 91q-12 0-16-10m81 8q13 2 14 12" fill="none" stroke-width="7"/>
      <circle cx="14" cy="81" r="5" fill="#ffbe6f"/><circle cx="109" cy="101" r="5" fill="#ffbe6f"/>
      <path d="M22 79c0-22 15-39 39-39s39 18 41 40l1 10q0 11-12 11H33Q20 101 21 89Z" fill="#f6a04d"/>
      <path d="M32 65q7-14 21-17" fill="none" stroke="#ffdea2" stroke-width="4"/>
      ${away ? `<path d="M44 70h38m-38 8h38m-33 8h29" fill="none" stroke="#af692f"/>` : `
        <rect x="29" y="64" width="62" height="27" rx="13" fill="#342e30"/>
        <path d="M43 77q4-6 8-1m17 0q4-6 8-1" stroke="#ffe5a3" stroke-width="3.5" fill="none"/>
        <circle cx="94" cy="91" r="2" fill="#ffe3a4" stroke="none"/>`}
      <path d="M43 102l-7 4 3 9m37-13 7 4-3 9" stroke-width="8"/>
      <path d="M38 113q-9-1-14 6 1 5 17 4l3-8zm42 0q11 0 14 8-5 3-16 2l-4-8z" fill="#48382e"/>
    </g>`);
}

export const dutyAgents = {
  terminal: terminal(false),
  terminalBack: terminal(true),
  courier: courier(false),
  courierBack: courier(true),
  envelope: illustration("0 0 42 32", `
    <rect x="2" y="4" width="38" height="25" rx="4" fill="#fff6da" stroke="#343a4f" stroke-width="2"/>
    <path d="m4 7 15 11q2 2 4 0L38 7" fill="none" stroke="#647798" stroke-width="2"/>`),
  badge: illustration("0 0 42 32", `
    <rect x="3" y="5" width="36" height="24" rx="4" fill="#fffaf0" stroke="#2e3d60" stroke-width="2"/>
    <rect x="16" y="2" width="10" height="6" rx="2" fill="#a0b5d9" stroke="#2e3d60" stroke-width="1.5"/>`),
};

// A panoramic night shift outside the studio window. Keep the centre quiet for the characters.
export const studioWindow = illustration("0 0 520 180", `
  <defs>
    <linearGradient id="sky" x1="0%" y1="0%" x2="0%" y2="100%" gradientUnits="objectBoundingBox">
      <stop stop-color="#172a50"/><stop offset="1" stop-color="#496fa2"/>
    </linearGradient>
    <pattern id="dots" width="7" height="7" patternUnits="userSpaceOnUse">
      <circle cx="1" cy="1" r=".65" fill="#cbdcf6" opacity=".48"/>
    </pattern>
  </defs>
  <rect width="520" height="180" fill="url(#sky)"/><rect width="520" height="180" fill="url(#dots)"/>
  <circle cx="415" cy="50" r="21" fill="#ffe3a5"/>
  <path d="M398 58q13 5 34-3m-32 9q13 3 27-2" stroke="#edc78b" stroke-width="2" fill="none"/>
  <g fill="#eff6fe"><circle cx="90" cy="55" r="1.8"/><circle cx="173" cy="22" r="1.5"/>
    <circle cx="295" cy="58" r="1.8"/><circle cx="350" cy="22" r="1.4"/><circle cx="457" cy="94" r="1.8"/>
    <path d="m126 29 1.5 5 5 1.5-5 1.5-1.5 5-1.5-5-5-1.5 5-1.5zm209 54 1 3.5 3.5 1-3.5 1-1 3.5-1-3.5-3.5-1 3.5-1z"/>
  </g>
  <path d="M24 83q46 20 92 5t87-47" stroke="#b2cbe9" stroke-dasharray="3 6" fill="none" opacity=".5"/>
  <circle cx="206" cy="39" r="2.5" fill="#e6edf5"/>
  <path d="m0 122 56-14 64 20 72-24 60 18 67-19 49 8 52-21 69 20 31-8v78H0Z" fill="#405b83"/>
  <path d="m0 145 74-19 60 8 58-12 80 20 80-18 50 7 75-28 43 6v71H0Z" fill="#324c71"/>
  <path d="M0 152h27v-15h17v15h6v-25h22v23h9v-12h10v18h43v-10h15v11h182v-7h38v-16h19v15h10v-29h20v24h19v-12h14v16h30v-9h24v11h15v35H0Z" fill="#263c5c"/>
  <path d="M62 126v-17m347 10v-13" stroke="#263c5c" stroke-width="2"/>
  <path d="M56 135h3m4 0h3m-10 6h3m346-12h3m4 0h3m-9 7h2m39 4h3" stroke="#ffdc9e" stroke-width="2"/>
`);
