<?php

namespace App\Http\Controllers;

use Illuminate\Http\Request;
use Illuminate\Support\Str;
use Illuminate\Support\Facades\Cache;

class WebSocketTicketController extends Controller
{
    /**
     * Called by the client via AJAX to get a short-lived ticket.
     * The client needs to be authenticated in Laravel to call this.
     */
    public function generateTicket(Request $request)
    {
        $user = $request->user();
        if (!$user) {
            // For testing purposes without full auth setup, you might fallback to a test user:
            // $userId = 1; 
            return response()->json(['error' => 'Unauthorized'], 401);
        }

        $userId = $user->id;

        // Generate a random string as the ticket
        $ticket = Str::random(40);

        // Store the ticket in cache for a very short duration (e.g. 30 seconds)
        // Key is ticket, Value is User ID.
        Cache::put('ws_ticket_' . $ticket, $userId, 30);

        return response()->json([
            'ticket' => $ticket,
            'ws_url' => env('WS_SERVER_URL', 'ws://localhost:8080/ws')
        ]);
    }

    /**
     * Called Internally by the Go server to validate the ticket.
     */
    public function validateTicket(Request $request)
    {
        // Enforce strong internal security here
        $authHeader = $request->header('Authorization');
        $internalSecret = 'Bearer ' . env('WS_INTERNAL_SECRET', 'super-secret-internal-key');
        
        if ($authHeader !== $internalSecret) {
            return response()->json(['error' => 'Unauthorized internal call'], 401);
        }

        $ticket = $request->input('ticket');
        if (!$ticket) {
            return response()->json(['error' => 'Ticket missing'], 400);
        }

        $cacheKey = 'ws_ticket_' . $ticket;
        $userId = Cache::get($cacheKey);

        if (!$userId) {
            return response()->json(['error' => 'Invalid or expired ticket'], 401);
        }

        // Invalidate the ticket immediately so it can only be used once
        Cache::forget($cacheKey);

        return response()->json(['user_id' => $userId]);
    }
}
