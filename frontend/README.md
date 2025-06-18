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

## Running the Application and Available Scripts

Once the setup is complete, you can use the following scripts:

### To Run in Development Mode:

1.  Execute the following command in the `frontend` directory:
    ```bash
    npm run dev
    ```
2.  Open your web browser and go to [http://localhost:5173](http://localhost:5173) (or the port specified in your terminal output).

    This mode is recommended for development as it provides hot reloading and detailed error messages.

### To Build for Production:

1.  Execute the following command in the `frontend` directory:
    ```bash
    npm run build
    ```
    This will create an optimized build of the application in the `frontend/dist` directory.

### To Preview the Production Build Locally:

1.  First, ensure you have built the application (see "To Build for Production").
2.  Then, execute the following command in the `frontend` directory:
    ```bash
    npm run preview
    ```
3.  Open your web browser and go to the URL provided in your terminal output (usually [http://localhost:4173](http://localhost:4173)).

    This is useful for testing the production build before deployment.

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
