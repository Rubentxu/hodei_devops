# Frontend Application for Task Orchestrator

This directory contains a React-based frontend application for interacting with the Task Orchestrator backend. It allows users to view, create, and stop tasks managed by the orchestrator via a WebSocket connection.

## Prerequisites

- Node.js (v16 or later recommended)
- npm (usually comes with Node.js)

## Setup

1.  **Navigate to the frontend directory:**
    ```bash
    cd frontend
    ```

2.  **Install dependencies:**
    ```bash
    npm install
    ```

## Available Scripts

In the `frontend` directory, you can run the following scripts:

### `npm run dev`

Runs the app in development mode.
Open [http://localhost:5173](http://localhost:5173) (or the port shown in your terminal) to view it in your browser.

The page will reload when you make changes.
You may also see any lint errors in the console.

### `npm run build`

Builds the app for production to the `dist` folder.
It correctly bundles React in production mode and optimizes the build for the best performance.

The build is minified and the filenames include the hashes.
Your app is ready to be deployed!

### `npm run preview`

Serves the production build from the `dist` folder locally.
This is a good way to test the production build before deploying.
Open the URL shown in your terminal (usually http://localhost:4173) to view it.

## Project Structure

-   `src/`: Contains the main source code for the React application.
    -   `components/`: UI components (e.g., `TaskList`, `TaskForm`, `TaskItem`).
    -   `context/`: React Context for global state management (`AppContext`).
    -   `hooks/`: Custom React hooks (e.g., `useWebSocket`).
    -   `App.jsx`: Main application component.
    -   `main.jsx`: Entry point of the application.
    -   `App.css`: Global styles for the `App` component.
    -   `index.css`: Global base styles.
-   `public/`: Static assets.
-   `dist/`: Production build output (after running `npm run build`).
-   `package.json`: Project dependencies and scripts.
-   `vite.config.js`: Vite configuration file.

## Backend Connection

The application connects to the backend WebSocket server at `ws://localhost:8080/ws` by default. This URL is defined in `src/hooks/useWebSocket.js`. If your backend runs on a different address or port, you will need to update this URL.
