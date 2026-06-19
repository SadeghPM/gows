<?php

namespace App\Providers;

use Illuminate\Support\ServiceProvider;
use Illuminate\Support\Facades\Broadcast;
use App\Broadcasting\GoWSBroadcaster;

class GoWSBroadcastServiceProvider extends ServiceProvider
{
    /**
     * Bootstrap any application services.
     */
    public function boot(): void
    {
        // Extend the Broadcast Manager to use the 'gows' driver
        Broadcast::extend('gows', function ($app, $config) {
            return new GoWSBroadcaster();
        });
    }

    /**
     * Register any application services.
     */
    public function register(): void
    {
        //
    }
}
